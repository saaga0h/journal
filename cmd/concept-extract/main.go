package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/saaga0h/journal/internal/config"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	var (
		repoPath   = flag.String("repo", "", "path to git repo (required)")
		days       = flag.Int("days", 1, "how many days back to look")
		hours      = flag.Int("hours", 0, "how many hours back (overrides days if set)")
		week       = flag.Bool("week", false, "extract previous calendar week (Mon-Sun UTC), overrides --days/--hours")
		deep       = flag.Bool("deep", false, "run second pass for theoretical territory")
		maxDiff    = flag.Int("max-diff", 12000, "max bytes of diff content to send")
		configPath = flag.String("config", "", "path to .env configuration file")
	)
	flag.Parse()

	log := logger.New()

	if *repoPath == "" {
		log.Fatal("--repo is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	logger.SetLevel(cfg.Log.Level)

	var since, until time.Time
	if *week {
		now := time.Now().UTC()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7 in ISO week numbering
		}
		daysFromMonday := weekday - 1
		thisMonday := now.AddDate(0, 0, -daysFromMonday).Truncate(24 * time.Hour)
		since = thisMonday.AddDate(0, 0, -7)
		until = thisMonday.Add(-time.Second) // last Sunday 23:59:59 UTC
	} else if *hours > 0 {
		since = time.Now().Add(-time.Duration(*hours) * time.Hour)
		until = time.Now()
	} else {
		since = time.Now().AddDate(0, 0, -*days)
		until = time.Now()
	}

	repoName := filepath.Base(*repoPath)

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)

	model := cfg.Ollama.ChatModel
	numCtx := cfg.Ollama.ChatNumCtx

	// Connect to MQTT once for all publishes
	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-concept-extract-%d", time.Now().UnixNano()),
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT")
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	// Enumerate active commit days in the window
	log.WithFields(map[string]interface{}{
		"repo":  *repoPath,
		"since": since.Format("2006-01-02"),
		"until": until.Format("2006-01-02"),
	}).Info("Enumerating commit days")

	commitDays, err := services.GetCommitDays(*repoPath, since, until)
	if err != nil {
		log.WithError(err).Fatal("Failed to enumerate commit days")
	}

	if len(commitDays) == 0 {
		log.Info("No commits found in the given time range")
		os.Exit(0)
	}

	log.WithField("days", len(commitDays)).Info("Found commit days — extracting per day")

	published := 0
	for _, day := range commitDays {
		dayStart := day
		dayEnd := day.Add(24*time.Hour - time.Second)

		log.WithField("day", dayStart.Format("2006-01-02")).Info("Extracting day")

		messages, err := services.GetCommitMessages(*repoPath, dayStart, dayEnd)
		if err != nil {
			log.WithError(err).WithField("day", dayStart.Format("2006-01-02")).Warn("Failed to get commit messages — skipping")
			continue
		}
		if strings.TrimSpace(messages) == "" {
			continue
		}

		diff, err := services.GetNonTestDiff(*repoPath, dayStart, dayEnd, *maxDiff)
		if err != nil {
			log.WithError(err).Warn("Could not get diff, continuing with messages only")
			diff = ""
		}

		gitContent := "=== COMMIT MESSAGES (all commits, oldest first) ===\n" +
			messages +
			"\n\n=== CODE CHANGES (non-test files, oldest first) ===\n" +
			diff

		first, err := services.ExtractConcepts(ollama, model, gitContent, numCtx)
		if err != nil {
			log.WithError(err).WithField("day", dayStart.Format("2006-01-02")).Warn("Concept extraction failed — skipping")
			continue
		}

		var second map[string]interface{}
		if *deep {
			second, err = services.DeepExtract(ollama, model, first, numCtx)
			if err != nil {
				log.WithError(err).Warn("Deep pass failed, continuing without")
			}
		}

		engineering, err := json.Marshal(first)
		if err != nil {
			log.WithError(err).Warn("Failed to marshal engineering results — skipping")
			continue
		}

		msg := mqttclient.EntryIngest{
			Envelope: mqttclient.Envelope{
				MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
				Source:    "concept-extract",
				Timestamp: time.Now(),
			},
			Repository:       repoName,
			SinceTimestamp:   dayStart,
			UntilTimestamp:   dayEnd,
			ExtractorVersion: "0.3.0",
			Engineering:      json.RawMessage(engineering),
			GitInput:         gitContent,
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
			log.WithError(err).WithField("day", dayStart.Format("2006-01-02")).Warn("Failed to publish — skipping")
			continue
		}

		log.WithFields(map[string]interface{}{
			"day":        dayStart.Format("2006-01-02"),
			"repository": repoName,
			"deep":       second != nil,
		}).Info("Published day entry")

		published++
	}

	log.WithFields(map[string]interface{}{
		"repository": repoName,
		"days_found": len(commitDays),
		"published":  published,
	}).Info("Extraction complete")
}
