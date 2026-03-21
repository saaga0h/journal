package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "", "Path to .env configuration file")
	flag.Parse()

	log := logger.New()

	if *configPath != "" {
		if err := godotenv.Load(*configPath); err != nil {
			log.WithError(err).Fatal("Failed to load config file")
		}
	} else {
		godotenv.Load()
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	logger.SetLevel(cfg.Log.Level)

	// Connect to database
	pool, err := database.Connect(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer pool.Close()

	// Run migrations
	if err := database.RunMigrations(pool); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	// Ollama service
	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)
	var ollamaMu sync.Mutex

	// MQTT client
	client, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-entry-ingest-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT broker")
	}
	defer client.Disconnect()
	client.SetLogger(log)

	threshold := float32(cfg.AssociationThreshold)

	// Subscribe to entry ingest topic
	if err := client.Subscribe(mqttclient.TopicEntriesIngest, func(payload []byte) {
		// Copy payload before goroutine — Paho reuses the buffer
		data := make([]byte, len(payload))
		copy(data, payload)

		go func() {
			var msg mqttclient.EntryIngest
			if err := json.Unmarshal(data, &msg); err != nil {
				log.WithError(err).Warn("Failed to unmarshal EntryIngest message")
				return
			}

			log.WithFields(logrus.Fields{
				"source":        msg.Source,
				"extractor_version": msg.ExtractorVersion,
			}).Info("Processing entry ingest")

			// Parse structured fields from raw JSON
			summary, concepts, theoTerritory := parseExtractorFields(msg.Engineering, msg.Theoretical)

			// Build embed text
			embedText := services.BuildEmbedText(msg.Engineering, msg.Theoretical)

			// Compute embedding — if fails, store without embedding
			var embedding []float32
			var embeddingVec pgvector.Vector
			ollamaMu.Lock()
			embedding, embedErr := ollama.Embed(embedText)
			ollamaMu.Unlock()

			if embedErr != nil {
				log.WithError(embedErr).Warn("Failed to compute embedding — storing entry without embedding")
			} else {
				embeddingVec = pgvector.NewVector(embedding)
			}

			// Build raw output from the full message
			rawOutput, _ := json.Marshal(msg)

			var untilTimestamp *time.Time
			if !msg.UntilTimestamp.IsZero() {
				t := msg.UntilTimestamp
				untilTimestamp = &t
			}

			entry := &database.JournalEntry{
				Source:           msg.Source,
				SinceTimestamp:       msg.SinceTimestamp,
				UntilTimestamp:       untilTimestamp,
				ExtractorVersion:     msg.ExtractorVersion,
				Engineering:          msg.Engineering,
				Theoretical:          msg.Theoretical,
				Summary:              summary,
				Concepts:             concepts,
				TheoreticalTerritory: theoTerritory,
				Embedding:            embeddingVec,
				GitInput:             msg.GitInput,
				RawOutput:            rawOutput,
			}

			entryID, err := database.InsertEntry(pool, entry)
			if err != nil {
				log.WithError(err).Error("Failed to insert journal entry")
				return
			}

			log.WithFields(logrus.Fields{
				"entry_id":   entryID,
				"source": msg.Source,
				"embedded":   embedErr == nil,
			}).Info("Journal entry stored")

			// Compute standing document associations if we have an embedding
			if embedErr == nil {
				standings, err := database.GetAllCurrentEmbeddings(pool)
				if err != nil {
					log.WithError(err).Warn("Failed to fetch standing document embeddings")
				} else {
					assocs := services.ComputeStandingAssociations(embedding, standings, threshold)
					for _, a := range assocs {
						if err := database.InsertEntryStandingAssociation(pool, entryID, a.StandingSlug, a.Similarity); err != nil {
							log.WithError(err).WithFields(logrus.Fields{
								"entry_id":      entryID,
								"standing_slug": a.StandingSlug,
							}).Warn("Failed to insert association")
						}
					}
					if len(assocs) > 0 {
						log.WithFields(logrus.Fields{
							"entry_id":     entryID,
							"associations": len(assocs),
						}).Info("Standing document associations recorded")
					}
				}
			}

			// Publish EntryCreated
			createdMsg := mqttclient.EntryCreated{
				Envelope: mqttclient.Envelope{
					MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
					Source:    "entry-ingest",
					Timestamp: time.Now(),
				},
				EntryID:    entryID,
				Source: msg.Source,
			}

			if err := client.Publish(mqttclient.TopicEntriesCreated, createdMsg); err != nil {
				log.WithError(err).Error("Failed to publish EntryCreated")
			} else {
				log.WithFields(logrus.Fields{
					"entry_id": entryID,
					"topic":    mqttclient.TopicEntriesCreated,
				}).Info("Published entry created notification")
			}
		}()
	}); err != nil {
		log.WithError(err).Fatal("Failed to subscribe to entries/ingest")
	}

	log.WithFields(logrus.Fields{
		"broker":    cfg.MQTT.BrokerURL,
		"threshold": threshold,
	}).Info("Entry ingest service ready — listening for entries")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("Shutting down entry ingest service")
}

// parseExtractorFields extracts summary, concepts, and theoretical territory
// from the raw engineering and theoretical JSON blocks.
func parseExtractorFields(engineering, theoretical json.RawMessage) (summary string, concepts []string, theoTerritory []string) {
	if len(engineering) > 0 && string(engineering) != "null" {
		var eng struct {
			Summary  string   `json:"summary"`
			Concepts []string `json:"concepts"`
		}
		if err := json.Unmarshal(engineering, &eng); err == nil {
			summary = eng.Summary
			concepts = eng.Concepts
		}
	}

	if len(theoretical) > 0 && string(theoretical) != "null" {
		var theo struct {
			TheoreticalTerritory []string `json:"theoretical_territory"`
		}
		if err := json.Unmarshal(theoretical, &theo); err == nil {
			theoTerritory = theo.TheoreticalTerritory
		}
	}

	return
}
