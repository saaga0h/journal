package main

import (
	"flag"

	"github.com/joho/godotenv"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
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

	threshold := float32(cfg.AssociationThreshold)

	// Find entries without embeddings
	entries, err := database.GetEntriesWithoutEmbedding(pool)
	if err != nil {
		log.WithError(err).Fatal("Failed to query entries without embeddings")
	}

	if len(entries) == 0 {
		log.Info("No entries without embeddings found")
		return
	}

	log.WithField("count", len(entries)).Info("Found entries without embeddings")

	// Fetch standing document embeddings once
	standings, err := database.GetAllCurrentEmbeddings(pool)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch standing document embeddings")
	}

	reembedded := 0
	for _, entry := range entries {
		embedText := services.BuildEmbedText(entry.Engineering, entry.Theoretical)
		if embedText == "" {
			log.WithField("entry_id", entry.ID).Warn("Empty embed text — skipping")
			continue
		}

		embedding, err := ollama.Embed(embedText)
		if err != nil {
			log.WithError(err).WithField("entry_id", entry.ID).Warn("Failed to compute embedding — skipping")
			continue
		}

		if err := database.UpdateEmbedding(pool, entry.ID, pgvector.NewVector(embedding)); err != nil {
			log.WithError(err).WithField("entry_id", entry.ID).Error("Failed to update embedding")
			continue
		}

		// Compute associations
		assocs := services.ComputeStandingAssociations(embedding, standings, threshold)
		for _, a := range assocs {
			if err := database.InsertEntryStandingAssociation(pool, entry.ID, a.StandingSlug, a.Similarity); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"entry_id":      entry.ID,
					"standing_slug": a.StandingSlug,
				}).Warn("Failed to insert association")
			}
		}

		log.WithFields(logrus.Fields{
			"entry_id":     entry.ID,
			"dimensions":   len(embedding),
			"associations": len(assocs),
		}).Info("Re-embedded entry")

		reembedded++
	}

	log.WithFields(logrus.Fields{
		"total":      len(entries),
		"reembedded": reembedded,
	}).Info("Re-embedding complete")
}
