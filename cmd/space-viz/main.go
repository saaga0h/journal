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
	"sync"

	"github.com/joho/godotenv"
	"github.com/saaga0h/journal/internal/config"
	"github.com/saaga0h/journal/internal/database"
	"github.com/saaga0h/journal/internal/markdown"
	"github.com/saaga0h/journal/internal/services"
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

	ollama := services.NewOllama(cfg.Ollama)
	ollama.SetLogger(log)
	var ollamaMu sync.Mutex

	// GET /health
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

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

	// GET /api/meta — data span info for slider initialisation
	http.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		span, err := database.GetDataSpanDays(pool)
		if err != nil {
			log.WithError(err).Error("Failed to get data span")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			SpanDays int `json:"span_days"`
		}{SpanDays: span})
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

	// GET /api/manifold?slug=X&days=N
	http.HandleFunc("/api/manifold", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		if slug == "" {
			http.Error(w, "slug parameter required", http.StatusBadRequest)
			return
		}

		windowDays := *days
		if d := r.URL.Query().Get("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				windowDays = parsed
			}
		}

		doc, err := database.GetCurrentStandingDocument(pool, slug)
		if err != nil {
			log.WithError(err).WithField("slug", slug).Error("Failed to fetch standing document")
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}

		chunks := markdown.ChunkMarkdown(doc.Content)
		if len(chunks) == 0 {
			http.Error(w, "document produced no chunks", http.StatusUnprocessableEntity)
			return
		}

		type chunkResult struct {
			Index     int       `json:"index"`
			Text      string    `json:"text"`
			Embedding []float32 `json:"embedding"`
		}
		var chunkResults []chunkResult

		for i, chunk := range chunks {
			embedText := services.TruncateForEmbed(doc.Title+": "+chunk, 24000)
			ollamaMu.Lock()
			emb, embErr := ollama.Embed(embedText)
			ollamaMu.Unlock()
			if embErr != nil {
				log.WithError(embErr).WithField("chunk", i).Warn("Failed to embed chunk, skipping")
				continue
			}
			chunkResults = append(chunkResults, chunkResult{
				Index:     i,
				Text:      truncateString(chunk, 200),
				Embedding: emb,
			})
		}

		if len(chunkResults) == 0 {
			http.Error(w, "all chunks failed to embed", http.StatusInternalServerError)
			return
		}

		chunkEmbeddings := make([][]float32, len(chunkResults))
		for i, cr := range chunkResults {
			chunkEmbeddings[i] = cr.Embedding
		}

		entries, err := database.GetRecentEntryEmbeddings(pool, windowDays)
		if err != nil {
			log.WithError(err).Error("Failed to fetch entry embeddings")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		type entryResult struct {
			EntryID          int64     `json:"entry_id"`
			Source           string    `json:"source"`
			SinceTimestamp   string    `json:"since_timestamp"`
			Concepts         []string  `json:"concepts"`
			Embedding        []float32 `json:"embedding"`
			NearestChunkDist float32   `json:"nearest_chunk_dist"`
		}
		entryResults := make([]entryResult, 0, len(entries))
		for _, e := range entries {
			entryResults = append(entryResults, entryResult{
				EntryID:          e.EntryID,
				Source:           e.Source,
				SinceTimestamp:   e.SinceTimestamp.Format("2006-01-02T15:04:05Z"),
				Concepts:         e.Concepts,
				Embedding:        e.Embedding.Slice(),
				NearestChunkDist: services.NearestChunkDistance(e.Embedding.Slice(), chunkEmbeddings),
			})
		}

		resp := struct {
			Slug    string        `json:"slug"`
			Title   string        `json:"title"`
			Chunks  []chunkResult `json:"chunks"`
			Entries []entryResult `json:"entries"`
		}{
			Slug:    doc.Slug,
			Title:   doc.Title,
			Chunks:  chunkResults,
			Entries: entryResults,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

// truncateString returns s truncated to maxLen runes with "…" appended if truncated.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
