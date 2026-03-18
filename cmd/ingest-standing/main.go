package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	filePath := flag.String("file", "", "Path to markdown standing document file (required)")
	slug := flag.String("slug", "", "Slug for the document (derived from filename if not provided)")
	title := flag.String("title", "", "Title for the document (extracted from first # heading if not provided)")
	configPath := flag.String("config", "", "Path to .env configuration file")
	flag.Parse()

	log := logger.New()

	if *filePath == "" {
		log.Fatal("--file is required")
	}

	// Load config
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

	// Read markdown file
	content, err := os.ReadFile(*filePath)
	if err != nil {
		log.WithError(err).WithField("file", *filePath).Fatal("Failed to read file")
	}

	// Derive slug from filename if not provided
	docSlug := *slug
	if docSlug == "" {
		docSlug = slugFromFilename(*filePath)
	}

	// Extract title from first # heading if not provided
	docTitle := *title
	if docTitle == "" {
		docTitle = titleFromContent(string(content))
	}
	if docTitle == "" {
		docTitle = docSlug // fallback
	}

	log.WithFields(map[string]interface{}{
		"file":  *filePath,
		"slug":  docSlug,
		"title": docTitle,
	}).Info("Ingesting standing document")

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

	// Compute embedding via Ollama
	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)

	embedding, err := ollama.Embed(string(content))
	if err != nil {
		log.WithError(err).Fatal("Failed to compute embedding — standing documents require embeddings")
	}

	// Store in database
	id, version, err := database.InsertStandingDocument(
		pool, docSlug, docTitle, string(content),
		pgvector.NewVector(embedding), *filePath,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to insert standing document")
	}

	log.WithFields(map[string]interface{}{
		"id":         id,
		"slug":       docSlug,
		"version":    version,
		"dimensions": len(embedding),
	}).Info("Standing document ingested")

	// Publish MQTT notification
	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-ingest-standing-%d", time.Now().UnixNano()),
	})
	if err != nil {
		log.WithError(err).Warn("Failed to connect to MQTT — document stored but notification not sent")
		return
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	msg := mqttclient.StandingUpdated{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "ingest-standing",
			Timestamp: time.Now(),
		},
		Slug:    docSlug,
		Version: version,
	}

	if err := mqttClient.Publish(mqttclient.TopicStandingUpdated, msg); err != nil {
		log.WithError(err).Warn("Failed to publish MQTT notification")
	} else {
		log.WithField("topic", mqttclient.TopicStandingUpdated).Info("Published standing document update")
	}
}

// slugFromFilename derives a slug from a filename.
// "Soul Speed.md" → "soul-speed", "gradient-lossy-functions.md" → "gradient-lossy-functions"
func slugFromFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// titleFromContent extracts the title from the first "# " heading in markdown.
func titleFromContent(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}
