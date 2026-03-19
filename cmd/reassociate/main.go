package main

import (
	"flag"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
	"github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "", "path to .env configuration file")
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

	threshold := float32(cfg.AssociationThreshold)

	// Fetch all entries that have embeddings
	entries, err := database.ListEntries(pool, database.ListEntriesOpts{Limit: 10000})
	if err != nil {
		log.WithError(err).Fatal("Failed to list entries")
	}

	// Filter to entries with embeddings
	var withEmbedding []database.JournalEntry
	for _, e := range entries {
		if len(e.Embedding.Slice()) > 0 {
			withEmbedding = append(withEmbedding, e)
		}
	}

	if len(withEmbedding) == 0 {
		log.Info("No entries with embeddings found")
		return
	}

	log.WithField("count", len(withEmbedding)).Info("Reassociating entries against current standing documents")

	// Fetch current standing document embeddings once
	standings, err := database.GetAllCurrentEmbeddings(pool)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch standing document embeddings")
	}

	log.WithField("standing_count", len(standings)).Info("Loaded current standing documents")

	done := 0
	for _, entry := range withEmbedding {
		embedding := entry.Embedding.Slice()

		// Delete existing associations
		if err := database.DeleteEntryStandingAssociations(pool, entry.ID); err != nil {
			log.WithError(err).WithField("entry_id", entry.ID).Error("Failed to delete old associations — skipping")
			continue
		}

		// Recompute against current standing docs
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
			"repository":   entry.Repository,
			"associations": len(assocs),
		}).Info("Reassociated entry")

		done++
	}

	log.WithFields(logrus.Fields{
		"total": len(withEmbedding),
		"done":  done,
	}).Info("Reassociation complete")
}
