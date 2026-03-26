package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/internal/webdav"
	"github.com/saaga0h/journal/pkg/logger"
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

	if cfg.WebDAV.BaseURL == "" {
		log.Fatal("WEBDAV_URL is required")
	}

	// Connect to database
	pool, err := database.Connect(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer pool.Close()

	if err := database.RunMigrations(pool); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	// WebDAV client
	dav := webdav.NewClient(cfg.WebDAV.BaseURL, cfg.WebDAV.Username, cfg.WebDAV.Password)
	dav.SetLogger(log)

	log.WithField("path", cfg.WebDAV.StandingPath).Info("Listing WebDAV standing documents")

	files, err := dav.List(cfg.WebDAV.StandingPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to list WebDAV directory")
	}

	log.WithField("count", len(files)).Info("Found files")

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)

	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-ingest-webdav-standing-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Warn("Failed to connect to MQTT — documents will be stored but notifications not sent")
	} else {
		defer mqttClient.Disconnect()
		mqttClient.SetLogger(log)
	}

	ingested, skipped, failed := 0, 0, 0

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

		existingHash, err := database.GetStandingDocumentHash(pool, docSlug)
		if err != nil {
			log.WithError(err).Warn("Failed to check existing hash — skipping")
			failed++
			continue
		}
		if existingHash == hash {
			log.Debug("Unchanged — skipping")
			skipped++
			continue
		}

		docTitle := titleFromContent(string(content))
		if docTitle == "" {
			docTitle = docSlug
		}

		embedding, err := ollama.Embed(services.TruncateForEmbed(string(content), 24000))
		if err != nil {
			log.WithError(err).Warn("Failed to compute embedding — skipping")
			failed++
			continue
		}

		id, version, err := database.InsertStandingDocument(
			pool, docSlug, docTitle, string(content),
			pgvector.NewVector(embedding), f.Path, hash,
		)
		if err != nil {
			log.WithError(err).Warn("Failed to insert standing document — skipping")
			failed++
			continue
		}

		log.WithFields(map[string]interface{}{
			"id":      id,
			"version": version,
		}).Info("Standing document ingested")

		if mqttClient != nil {
			msg := mqttclient.StandingUpdated{
				Envelope: mqttclient.Envelope{
					MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
					Source:    "ingest-webdav-standing",
					Timestamp: time.Now(),
				},
				Slug:    docSlug,
				Version: version,
			}
			if err := mqttClient.Publish(mqttclient.TopicStandingUpdated, msg); err != nil {
				log.WithError(err).Warn("Failed to publish MQTT notification")
			}
		}

		ingested++
	}

	log.WithFields(map[string]interface{}{
		"total":    len(files),
		"ingested": ingested,
		"skipped":  skipped,
		"failed":   failed,
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

func titleFromContent(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}
