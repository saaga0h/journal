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
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	timeoutSecs := flag.Int("timeout-seconds", 30, "seconds to wait for Minerva response")
	windowDays  := flag.Int("window-days", 28, "trend detection lookback window in days")
	configPath  := flag.String("config", "", "path to .env configuration file")
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

	pool, err := database.Connect(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer pool.Close()

	if err := database.RunMigrations(pool); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	client, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-brief-assemble-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT broker")
	}
	defer client.Disconnect()
	client.SetLogger(log)

	threshold := float32(cfg.BriefRelevanceThreshold)

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)
	var ollamaMu sync.Mutex

	// pendingSessions tracks active brief sessions awaiting Minerva response.
	// key = session_id, value = trigger time
	var mu sync.Mutex
	pendingSessions := make(map[string]time.Time)

	// Subscribe to Minerva response topic
	if err := client.Subscribe(mqttclient.TopicMinervaResponse, func(payload []byte) {
		data := make([]byte, len(payload))
		copy(data, payload)

		go func() {
			var resp struct {
				SessionID    string  `json:"session_id"`
				ArticleURL   string  `json:"article_url"`
				ArticleTitle string  `json:"article_title"`
				Score        float32 `json:"score"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				log.WithError(err).Warn("Failed to unmarshal Minerva response")
				return
			}

			mu.Lock()
			triggerTime, ok := pendingSessions[resp.SessionID]
			if ok {
				delete(pendingSessions, resp.SessionID)
			}
			mu.Unlock()

			if !ok {
				log.WithField("session_id", resp.SessionID).Warn("Received Minerva response for unknown session")
				return
			}

			var result mqttclient.BriefResult
			result.Envelope = mqttclient.Envelope{
				MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
				Source:    "brief-assemble",
				Timestamp: time.Now(),
			}
			result.SessionID = resp.SessionID
			result.TriggerTime = triggerTime

			if resp.Score >= threshold {
				result.Silence = false
				result.ArticleURL = resp.ArticleURL
				result.ArticleTitle = resp.ArticleTitle
				result.Reason = fmt.Sprintf("relevance score %.2f above threshold %.2f", resp.Score, threshold)
				log.WithFields(logrus.Fields{
					"session_id": resp.SessionID,
					"url":        resp.ArticleURL,
					"score":      resp.Score,
				}).Info("Brief: surfacing article")
			} else {
				result.Silence = true
				result.SilenceReason = fmt.Sprintf("best candidate score %.2f below threshold %.2f", resp.Score, threshold)
				log.WithField("session_id", resp.SessionID).Info("Brief: silence — below threshold")
			}

			if err := client.Publish(mqttclient.TopicBriefResult, result); err != nil {
				log.WithError(err).Error("Failed to publish brief result")
			}

			// Record to brief_history
			score := resp.Score
			record := &database.BriefHistoryRecord{
				SessionID:   resp.SessionID,
				TriggeredAt: triggerTime,
				ArticleURL:  result.ArticleURL,
				ArticleTitle: result.ArticleTitle,
			}
			if !result.Silence {
				record.RelevanceScore = &score
			} else {
				record.SilenceReason = result.SilenceReason
			}
			if _, err := database.InsertBriefHistory(pool, record); err != nil {
				log.WithError(err).Warn("Failed to record brief history")
			}
		}()
	}); err != nil {
		log.WithError(err).Fatal("Failed to subscribe to Minerva response topic")
	}

	// Subscribe to brief trigger
	if err := client.Subscribe(mqttclient.TopicBriefTrigger, func(payload []byte) {
		data := make([]byte, len(payload))
		copy(data, payload)

		go func() {
			triggerTime := time.Now()
			sessionID := fmt.Sprintf("%x", triggerTime.UnixNano())

			log.WithField("session_id", sessionID).Info("Brief trigger received — computing manifold profile")

			// Fetch recent entries with raw embeddings
			entries, err := database.GetRecentEntryEmbeddings(pool, *windowDays)
			if err != nil {
				log.WithError(err).Error("Failed to fetch entry embeddings")
				publishSilence(client, log, sessionID, triggerTime, "trend_error")
				return
			}

			if len(entries) < 3 {
				log.WithField("entry_count", len(entries)).Info("Brief: silence — insufficient trend data")
				publishSilence(client, log, sessionID, triggerTime, "insufficient_trend_data")
				return
			}

			// Fetch standing doc contents and compute manifold chunk embeddings
			docs, err := database.GetCurrentStandingContents(pool)
			if err != nil {
				log.WithError(err).Error("Failed to fetch standing doc contents")
				publishSilence(client, log, sessionID, triggerTime, "trend_error")
				return
			}

			slugChunks, err := services.ComputeManifoldChunks(docs, ollama, &ollamaMu, log)
			if err != nil {
				log.WithError(err).Error("Failed to compute manifold chunks")
				publishSilence(client, log, sessionID, triggerTime, "trend_error")
				return
			}

			manifoldProfile := services.ManifoldProximityProfile(entries, slugChunks, 0.3, 14.0)

			// Track session for timeout handling
			mu.Lock()
			pendingSessions[sessionID] = triggerTime
			mu.Unlock()

			// Publish Minerva query
			query := struct {
				SessionID       string             `json:"session_id"`
				ManifoldProfile map[string]float32 `json:"manifold_profile"`
				TopK            int                `json:"top_k"`
				ResponseTopic   string             `json:"response_topic"`
			}{
				SessionID:       sessionID,
				ManifoldProfile: map[string]float32(manifoldProfile),
				TopK:            5,
				ResponseTopic:   mqttclient.TopicMinervaResponse,
			}

			if err := client.Publish(mqttclient.TopicMinervaQuery, query); err != nil {
				log.WithError(err).Error("Failed to publish Minerva query")
				mu.Lock()
				delete(pendingSessions, sessionID)
				mu.Unlock()
				publishSilence(client, log, sessionID, triggerTime, "minerva_publish_error")
				return
			}

			log.WithField("session_id", sessionID).Info("Minerva query published — waiting for response")

			// Timeout goroutine
			go func() {
				time.Sleep(time.Duration(*timeoutSecs) * time.Second)
				mu.Lock()
				_, stillPending := pendingSessions[sessionID]
				if stillPending {
					delete(pendingSessions, sessionID)
				}
				mu.Unlock()

				if stillPending {
					log.WithField("session_id", sessionID).Warn("Brief: silence — Minerva timeout")
					publishSilence(client, log, sessionID, triggerTime, "minerva_timeout")
					if _, err := database.InsertBriefHistory(pool, &database.BriefHistoryRecord{
						SessionID:     sessionID,
						TriggeredAt:   triggerTime,
						SilenceReason: "minerva_timeout",
					}); err != nil {
						log.WithError(err).Warn("Failed to record brief history for timeout")
					}
				}
			}()
		}()
	}); err != nil {
		log.WithError(err).Fatal("Failed to subscribe to brief trigger topic")
	}

	log.WithFields(logrus.Fields{
		"broker":    cfg.MQTT.BrokerURL,
		"threshold": threshold,
	}).Info("Brief assembler ready — listening for triggers")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("Shutting down brief assembler")
}

func publishSilence(client *mqttclient.Client, log *logrus.Logger, sessionID string, triggerTime time.Time, reason string) {
	result := mqttclient.BriefResult{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "brief-assemble",
			Timestamp: time.Now(),
		},
		SessionID:     sessionID,
		Silence:       true,
		SilenceReason: reason,
		TriggerTime:   triggerTime,
	}
	if err := client.Publish(mqttclient.TopicBriefResult, result); err != nil {
		log.WithError(err).Error("Failed to publish silence result")
	}
}
