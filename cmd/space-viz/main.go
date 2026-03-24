package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	"github.com/saaga0h/journal/pkg/logger"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	port        := flag.Int("port", 8765, "HTTP port")
	days        := flag.Int("days", 90, "default lookback window in days")
	openBrowser := flag.Bool("open", false, "open browser on startup (macOS)")
	configPath  := flag.String("config", "", "path to .env file")
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

	// GET /api/points?days=N
	http.HandleFunc("/api/points", func(w http.ResponseWriter, r *http.Request) {
		windowDays := *days
		if d := r.URL.Query().Get("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				windowDays = parsed
			}
		}
		points, err := database.GetRecentEntriesInStandingSpace(pool, windowDays)
		if err != nil {
			log.WithError(err).Error("Failed to fetch entries in standing space")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(points)
	})

	// GET /api/standing
	http.HandleFunc("/api/standing", func(w http.ResponseWriter, r *http.Request) {
		docs, err := database.ListCurrentStandingDocuments(pool)
		if err != nil {
			log.WithError(err).Error("Failed to list standing documents")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		type standingInfo struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		}
		result := make([]standingInfo, len(docs))
		for i, d := range docs {
			result[i] = standingInfo{Slug: d.Slug, Title: d.Title}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Serve embedded static files at /
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.WithError(err).Fatal("Failed to create sub filesystem")
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", *port)
	url  := fmt.Sprintf("http://localhost:%d", *port)
	log.WithField("url", url).Info("Starting space-viz")
	fmt.Printf("space-viz listening on %s\n", url)

	if *openBrowser && runtime.GOOS == "darwin" {
		exec.Command("open", url).Start()
	}

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.WithError(err).Fatal("Server failed")
	}
}
