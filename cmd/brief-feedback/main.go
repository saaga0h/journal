package main

import (
	"flag"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	sessionID  := flag.String("session-id", "", "brief session ID to record feedback for (required)")
	action     := flag.String("action", "", "feedback action: read or skip (required)")
	configPath := flag.String("config", "", "path to .env configuration file")
	flag.Parse()

	log := logger.New()

	if *sessionID == "" {
		log.Fatal("--session-id is required")
	}
	if *action != "read" && *action != "skip" {
		log.Fatal("--action must be 'read' or 'skip'")
	}

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

	if err := database.InsertBriefFeedback(pool, *sessionID, *action); err != nil {
		log.WithError(err).Fatal("Failed to record feedback")
	}

	log.WithFields(map[string]interface{}{
		"session_id": *sessionID,
		"action":     *action,
	}).Info("Brief feedback recorded")
}
