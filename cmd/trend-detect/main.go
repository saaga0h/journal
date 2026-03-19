package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	mqttclient "github.com/saaga0h/journal/internal/mqtt"
	"github.com/saaga0h/journal/internal/services"
	"github.com/saaga0h/journal/pkg/logger"
)

func main() {
	windowDays        := flag.Int("window-days", 28, "lookback window in days")
	glfK              := flag.Float64("glf-k", 0.3, "GLF steepness parameter")
	glfMidpointDays   := flag.Int("glf-midpoint-days", 14, "age in days where GLF weight = 0.5")
	exceptionThreshold := flag.Float64("exception-threshold", 0.5, "minimum cosine distance from centroid to flag an exception")
	publish           := flag.Bool("publish", true, "publish TrendResult to MQTT (false = print JSON to stdout)")
	configPath        := flag.String("config", "", "path to .env configuration file")
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

	// Fetch recent entries with embeddings
	entries, err := database.GetRecentEntriesWithEmbeddings(pool, *windowDays)
	if err != nil {
		log.WithError(err).Fatal("Failed to fetch recent entries")
	}

	if len(entries) < 3 {
		log.WithField("entry_count", len(entries)).Info("Insufficient data for trend detection — need at least 3 entries")
		os.Exit(0)
	}

	// Build GLF-weighted embeddings
	now := time.Now()
	embeddings := make([][]float32, 0, len(entries))
	weights := make([]float64, 0, len(entries))

	for _, e := range entries {
		raw := e.Embedding.Slice()
		if len(raw) == 0 {
			continue
		}
		ageDays := now.Sub(e.SinceTimestamp).Hours() / 24.0
		w := services.GLFWeight(ageDays, *glfK, float64(*glfMidpointDays))
		embeddings = append(embeddings, raw)
		weights = append(weights, w)
	}

	centroid, err := services.WeightedCentroid(embeddings, weights)
	if err != nil {
		log.WithError(err).Fatal("Failed to compute weighted centroid")
	}

	// Detect exceptions: entries distant from centroid AND close to a standing doc
	// that hasn't been recently activated by the trend.
	recentSlugs, err := database.GetRecentlyActivatedStandingSlugs(pool, *glfMidpointDays)
	if err != nil {
		log.WithError(err).Warn("Failed to fetch recently activated standing slugs — skipping exception detection")
	}
	recentSlugSet := make(map[string]bool, len(recentSlugs))
	for _, s := range recentSlugs {
		recentSlugSet[s] = true
	}

	standings, err := database.GetAllCurrentEmbeddings(pool)
	if err != nil {
		log.WithError(err).Warn("Failed to fetch standing document embeddings — skipping exception detection")
	}

	var exceptions []mqttclient.TrendException
	for _, e := range entries {
		raw := e.Embedding.Slice()
		if len(raw) == 0 {
			continue
		}
		centroidSim := services.CosineSimilarity(raw, centroid)
		centroidDist := 1.0 - centroidSim
		if float64(centroidDist) < *exceptionThreshold {
			continue
		}
		// Entry is distant from centroid — check if it's close to an inactive standing doc
		for _, sd := range standings {
			if recentSlugSet[sd.Slug] {
				continue
			}
			sdRaw := sd.Embedding.Slice()
			if len(sdRaw) == 0 {
				continue
			}
			standingSim := services.CosineSimilarity(raw, sdRaw)
			if standingSim > float32(cfg.AssociationThreshold) {
				exceptions = append(exceptions, mqttclient.TrendException{
					EntryID:               e.ID,
					ActivatedStandingSlug: sd.Slug,
					CentroidDistance:      centroidDist,
					StandingSimilarity:    standingSim,
				})
			}
		}
	}

	result := mqttclient.TrendResult{
		Envelope: mqttclient.Envelope{
			MessageID: fmt.Sprintf("%x", time.Now().UnixNano()),
			Source:    "trend-detect",
			Timestamp: time.Now(),
		},
		TrendVector: centroid,
		EntryCount:  len(embeddings),
		WindowDays:  *windowDays,
		Exceptions:  exceptions,
		ComputedAt:  time.Now(),
	}

	log.WithFields(map[string]interface{}{
		"entry_count":    result.EntryCount,
		"window_days":    result.WindowDays,
		"exceptions":     len(exceptions),
		"centroid_mag":   vectorMagnitude(centroid),
	}).Info("Trend computed")

	if !*publish {
		// Print human-readable concept summary before the raw JSON
		if len(standings) > 0 {
			type scored struct {
				slug  string
				title string
				sim   float32
			}
			var ranked []scored
			for _, sd := range standings {
				sdRaw := sd.Embedding.Slice()
				if len(sdRaw) == 0 {
					continue
				}
				ranked = append(ranked, scored{sd.Slug, sd.Title, services.CosineSimilarity(centroid, sdRaw)})
			}
			// Sort descending by similarity
			for i := 0; i < len(ranked); i++ {
				for j := i + 1; j < len(ranked); j++ {
					if ranked[j].sim > ranked[i].sim {
						ranked[i], ranked[j] = ranked[j], ranked[i]
					}
				}
			}
			fmt.Fprintf(os.Stdout, "\nTrend → standing document similarity:\n")
			for _, r := range ranked {
				fmt.Fprintf(os.Stdout, "  %-40s  %.3f  %s\n", r.slug, r.sim, r.title)
			}
			fmt.Fprintln(os.Stdout)
		}

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

func vectorMagnitude(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}
