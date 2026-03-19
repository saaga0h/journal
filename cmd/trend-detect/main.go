package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
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

	// Fetch entries as points in standing-doc space
	points, err := database.GetRecentEntriesInStandingSpace(pool, *windowDays)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch entries in standing space")
	}

	if len(points) < 3 {
		log.WithField("entry_count", len(points)).Info("Insufficient data for trend detection — need at least 3 entries with associations")
		os.Exit(0)
	}

	// Collect all slugs present across entries
	slugSet := make(map[string]bool)
	for _, pt := range points {
		for slug := range pt.Coords {
			slugSet[slug] = true
		}
	}
	allSlugs := make([]string, 0, len(slugSet))
	for s := range slugSet {
		allSlugs = append(allSlugs, s)
	}
	sort.Strings(allSlugs)

	midpoint := float64(*glfMidpointDays)
	gravity := services.GLFWeightedGravityProfile(points, allSlugs, *glfK, midpoint)
	soulSpeed := services.SoulSpeedProfile(points, *glfK, midpoint)
	spread := services.ClusterSpread(points, allSlugs, gravity)

	summary := buildHumanSummary(gravity, soulSpeed, spread, len(points), *windowDays)

	result := mqttclient.TrendResult{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "trend-detect",
			Timestamp: time.Now(),
		},
		GravityProfile: map[string]float32(gravity),
		SoulSpeed:      soulSpeed,
		ClusterSpread:  spread,
		EntryCount:     len(points),
		WindowDays:     *windowDays,
		ComputedAt:     time.Now(),
		HumanSummary:   summary,
	}

	log.WithFields(map[string]interface{}{
		"entry_count":    result.EntryCount,
		"window_days":    result.WindowDays,
		"soul_speed":     soulSpeed,
		"cluster_spread": spread,
	}).Info("Trend computed in standing space")

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

// buildHumanSummary renders a multi-line text description of the trend.
func buildHumanSummary(gravity services.GravityProfile, soulSpeed, spread float32, entryCount, windowDays int) string {
	type scored struct {
		slug string
		val  float32
	}
	ranked := make([]scored, 0, len(gravity))
	for slug, val := range gravity {
		ranked = append(ranked, scored{slug, val})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].val > ranked[j].val })

	s := fmt.Sprintf("Trend (%d entries, %dd window):\n", entryCount, windowDays)

	s += "  Top gravity: "
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

	s += fmt.Sprintf("  Soul Speed:   %.3f  (%s)\n", soulSpeed, soulSpeedLabel(soulSpeed))

	spreadLabel := "moderate"
	if spread < 0.03 {
		spreadLabel = "tight cluster"
	} else if spread > 0.08 {
		spreadLabel = "dispersed"
	}
	s += fmt.Sprintf("  Spread:       %.3f  (%s)", spread, spreadLabel)

	return s
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
