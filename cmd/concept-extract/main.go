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

	log.WithFields(map[string]interface{}{
		"repo":  *repoPath,
		"since": since.Format("2006-01-02 15:04"),
		"until": until.Format("2006-01-02 15:04"),
	}).Info("Fetching commits")

	messages, err := services.GetCommitMessages(*repoPath, since, until)
	if err != nil {
		log.WithError(err).Fatal("Failed to get commit messages")
	}

	if strings.TrimSpace(messages) == "" {
		log.Info("No commits found in the given time range")
		os.Exit(0)
	}

	diff, err := services.GetNonTestDiff(*repoPath, since, until, *maxDiff)
	if err != nil {
		log.WithError(err).Warn("Could not get diff, continuing with messages only")
		diff = ""
	}

	gitContent := "=== COMMIT MESSAGES (all commits, oldest first) ===\n" +
		messages +
		"\n\n=== CODE CHANGES (non-test files, oldest first) ===\n" +
		diff

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)

	model := cfg.Ollama.ChatModel
	numCtx := cfg.Ollama.ChatNumCtx

	log.WithFields(map[string]interface{}{
		"model":   model,
		"num_ctx": numCtx,
		"chars":   len(gitContent),
	}).Info("Running concept extraction")

	first, err := services.ExtractConcepts(ollama, model, gitContent, numCtx)
	if err != nil {
		log.WithError(err).Fatal("Concept extraction failed")
	}

	var second map[string]interface{}
	if *deep {
		log.Info("Running deep pass")
		second, err = services.DeepExtract(ollama, model, first, numCtx)
		if err != nil {
			log.WithError(err).Warn("Deep pass failed, continuing without")
		}
	}

	// Build MQTT message
	repoName := filepath.Base(*repoPath)

	engineering, err := json.Marshal(first)
	if err != nil {
		log.WithError(err).Fatal("Failed to marshal engineering results")
	}

	msg := mqttclient.EntryIngest{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "concept-extract",
			Timestamp: time.Now(),
		},
		Repository:       repoName,
		SinceTimestamp:   since,
		UntilTimestamp:   until,
		ExtractorVersion: "0.2.0",
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

	// Connect and publish
	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-concept-extract-%d", time.Now().UnixNano()),
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT")
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	if err := mqttClient.Publish(mqttclient.TopicEntriesIngest, msg); err != nil {
		log.WithError(err).Fatal("Failed to publish to MQTT")
	}

	log.WithFields(map[string]interface{}{
		"topic":      mqttclient.TopicEntriesIngest,
		"repository": repoName,
		"deep":       second != nil,
	}).Info("Published extraction results")
}
