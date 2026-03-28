package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	windowDays      := flag.Int("window-days", 28, "lookback window in days")
	glfK            := flag.Float64("glf-k", 0.3, "GLF steepness parameter")
	glfMidpointDays := flag.Int("glf-midpoint-days", 14, "age in days where GLF weight = 0.5")
	publish         := flag.Bool("publish", true, "publish TrendResult to MQTT (false = print to stdout)")
	configPath      := flag.String("config", "", "path to .env configuration file")
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

	// Fetch recent entries with raw embeddings for manifold proximity computation
	entries, err := database.GetRecentEntryEmbeddings(pool, *windowDays)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch entry embeddings")
	}

	if len(entries) < 3 {
		log.WithField("entry_count", len(entries)).Info("Insufficient data for trend detection — need at least 3 entries with embeddings")
		os.Exit(0)
	}

	// Fetch standing doc contents for chunk embedding
	docs, err := database.GetCurrentStandingContents(pool)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch standing doc contents")
	}

	// Embed chunks for all standing docs via Ollama (serialized)
	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)
	var ollamaMu sync.Mutex

	slugChunks, err := services.ComputeManifoldChunks(docs, ollama, &ollamaMu, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to compute manifold chunks")
	}

	midpoint := float64(*glfMidpointDays)
	profile := services.ManifoldProximityProfile(entries, slugChunks, *glfK, midpoint)
	soulSpeed := services.ManifoldSoulSpeed(entries, slugChunks, *glfK, midpoint)

	// Fetch entries in standing space for concept extraction (still association-based)
	points, err := database.GetRecentEntriesInStandingSpace(pool, *windowDays)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch entries in standing space")
	}

	trending := services.TrendingConcepts(points, *glfK, midpoint, 7)
	unexpected := services.UnexpectedConceptsFromManifold(entries, slugChunks, *glfK, midpoint, 5)

	summary := buildHumanSummary(profile, soulSpeed, trending, unexpected, len(entries), *windowDays)

	result := mqttclient.TrendResult{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "trend-detect",
			Timestamp: time.Now(),
		},
		ManifoldProfile:    map[string]float32(profile),
		SoulSpeed:          soulSpeed,
		TrendingConcepts:   trending,
		UnexpectedConcepts: unexpected,
		EntryCount:         len(entries),
		WindowDays:         *windowDays,
		ComputedAt:         time.Now(),
		HumanSummary:       summary,
	}

	log.WithFields(map[string]interface{}{
		"entry_count": result.EntryCount,
		"window_days": result.WindowDays,
		"soul_speed":  soulSpeed,
		"top_manifold": topManifold(profile),
	}).Info("Trend computed via manifold proximity")

	if !*publish {
		fmt.Fprintln(os.Stdout, summary)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			log.WithError(err).Fatal("Failed to encode trend result")
		}
		return
	}

	mqttClient, err := mqttclient.NewClient(mqttclient.ClientConfig{
		BrokerURL: cfg.MQTT.BrokerURL,
		ClientID:  fmt.Sprintf("journal-trend-detect-%d", time.Now().UnixNano()),
		Username:  cfg.MQTT.Username,
		Password:  cfg.MQTT.Password,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MQTT")
	}
	defer mqttClient.Disconnect()
	mqttClient.SetLogger(log)

	if err := mqttClient.Publish(mqttclient.TopicTrendCurrent, result); err != nil {
		log.WithError(err).Fatal("Failed to publish trend result")
	}
	log.WithField("topic", mqttclient.TopicTrendCurrent).Info("Published trend result")
}

// buildHumanSummary renders a multi-line text description of the manifold trend.
func buildHumanSummary(profile services.ManifoldProfile, soulSpeed float32, trending, unexpected []string, entryCount, windowDays int) string {
	type scored struct {
		slug string
		val  float32
	}
	ranked := make([]scored, 0, len(profile))
	for slug, val := range profile {
		ranked = append(ranked, scored{slug, val})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].val > ranked[j].val })

	s := fmt.Sprintf("Trend (%d entries, %dd window):\n", entryCount, windowDays)

	s += "  Top manifolds: "
	limit := 3
	if len(ranked) < limit {
		limit = len(ranked)
	}
	for i := 0; i < limit; i++ {
		if i > 0 {
			s += "  "
		}
		s += fmt.Sprintf(" %s %.3f", ranked[i].slug, ranked[i].val)
	}
	s += "\n"

	s += fmt.Sprintf("  Soul Speed:    %.3f  (%s)\n", soulSpeed, soulSpeedLabel(soulSpeed))

	if len(trending) > 0 {
		s += fmt.Sprintf("  Trending:      %s\n", joinStrings(trending, " · "))
	}
	if len(unexpected) > 0 {
		s += fmt.Sprintf("  Unexpected:    %s", joinStrings(unexpected, " · "))
	}

	return s
}

// topManifold returns the slug with highest proximity for log output.
func topManifold(profile services.ManifoldProfile) string {
	var best string
	var bestVal float32
	for slug, val := range profile {
		if val > bestVal {
			bestVal = val
			best = slug
		}
	}
	return best
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func soulSpeedLabel(v float32) string {
	switch {
	case v >= 0.65:
		return "high aliveness"
	case v >= 0.55:
		return "moderate aliveness"
	case v >= 0.45:
		return "low aliveness"
	default:
		return "dormant"
	}
}
