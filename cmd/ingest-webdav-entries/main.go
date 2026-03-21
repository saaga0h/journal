package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/internal/webdav"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	deep := flag.Bool("deep", false, "Run second pass for theoretical territory")
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

	if cfg.WebDAV.BaseURL == "" {
		log.Fatal("WEBDAV_URL is required")
	}

	// Connect to database (for skip state)
	pool, err := database.Connect(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer pool.Close()

	if err := database.RunMigrations(pool); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	// Connect to MQTT
	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-ingest-webdav-entries-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT")
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	dav := webdav.NewClient(cfg.WebDAV.BaseURL, cfg.WebDAV.Username, cfg.WebDAV.Password)
	dav.SetLogger(log)

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)

	log.WithField("path", cfg.WebDAV.EntriesPath).Info("Listing WebDAV entries")

	files, err := dav.List(cfg.WebDAV.EntriesPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to list WebDAV directory")
	}

	log.WithField("count", len(files)).Info("Found files")

	published, skipped, failed := 0, 0, 0

	for _, f := range files {
		docSlug := slugFromFilename(f.Name)
		log := log.WithField("file", f.Name).WithField("slug", docSlug)

		content, err := dav.Get(f.Path)
		if err != nil {
			log.WithError(err).Warn("Failed to fetch file — skipping")
			failed++
			continue
		}

		hash := fmt.Sprintf("%x", sha256.Sum256(content))

		existingHash, found, err := database.GetWebDAVState(pool, f.Path)
		if err != nil {
			log.WithError(err).Warn("Failed to check ingest state — skipping")
			failed++
			continue
		}
		if found && existingHash == hash {
			log.Debug("Unchanged — skipping")
			skipped++
			continue
		}

		// Parse date from content; fall back to WebDAV last modified
		docDate, dateFound := services.ParseDocumentDate(string(content))
		if !dateFound {
			if !f.LastModified.IsZero() {
				docDate = f.LastModified
				log.Debug("No date in content — using WebDAV last modified")
			} else {
				docDate = time.Now()
				log.Warn("No date found — using current time")
			}
		}

		dayStart := time.Date(docDate.Year(), docDate.Month(), docDate.Day(), 0, 0, 0, 0, time.UTC)
		dayEnd := dayStart.Add(24*time.Hour - time.Second)

		// Extract concepts (same pipeline as concept-extract)
		first, err := services.ExtractConcepts(ollama, cfg.Ollama.ChatModel, string(content), cfg.Ollama.ChatNumCtx)
		if err != nil {
			log.WithError(err).Warn("Concept extraction failed — skipping")
			failed++
			continue
		}

		var second map[string]interface{}
		if *deep {
			second, err = services.DeepExtract(ollama, cfg.Ollama.ChatModel, first, cfg.Ollama.ChatNumCtx)
			if err != nil {
				log.WithError(err).Warn("Deep pass failed — continuing without")
			}
		}

		engineering, err := json.Marshal(first)
		if err != nil {
			log.WithError(err).Warn("Failed to marshal extraction results — skipping")
			failed++
			continue
		}

		msg := mqttclient.EntryIngest{
			Envelope: mqttclient.Envelope{
				MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
				Source:    "webdav-entry",
				Timestamp: time.Now(),
			},
			Repository:       docSlug,
			SinceTimestamp:   dayStart,
			UntilTimestamp:   dayEnd,
			ExtractorVersion: "0.1.0",
			Engineering:      json.RawMessage(engineering),
		}

		if second != nil {
			theoretical, err := json.Marshal(second)
			if err != nil {
				log.WithError(err).Warn("Failed to marshal theoretical results")
			} else {
				msg.Theoretical = json.RawMessage(theoretical)
			}
		}

		if err := mqttClient.Publish(mqttclient.TopicEntriesIngest, msg); err != nil {
			log.WithError(err).Warn("Failed to publish — skipping")
			failed++
			continue
		}

		// Only update state after successful publish
		if err := database.UpsertWebDAVState(pool, f.Path, hash); err != nil {
			log.WithError(err).Warn("Failed to update ingest state — will retry next run")
		}

		log.WithField("date", dayStart.Format("2006-01-02")).Info("Published entry")
		published++
	}

	log.WithFields(map[string]interface{}{
		"total":     len(files),
		"published": published,
		"skipped":   skipped,
		"failed":    failed,
	}).Info("Done")
}

func slugFromFilename(filename string) string {
	base := path.Base(filename)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}
