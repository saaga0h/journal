package services

import (
	"fmt"
	"sync"

	"github.com/saaga0h/journal/internal/database"
	markdown "github.com/saaga0h/journal/internal/markdown"
	"github.com/sirupsen/logrus"
)

// ComputeManifoldChunks embeds the chunks of each standing doc via Ollama and returns
// a []SlugChunks ready for ManifoldProximityProfile and ManifoldSoulSpeed.
// Ollama calls are serialized via mu to prevent concurrent-request timeouts.
// Docs whose content produces no chunks (too short) are silently skipped.
func ComputeManifoldChunks(
	docs []database.StandingDocContent,
	ollama *Ollama,
	mu *sync.Mutex,
	log *logrus.Logger,
) ([]SlugChunks, error) {
	result := make([]SlugChunks, 0, len(docs))

	for _, doc := range docs {
		chunks := markdown.ChunkMarkdown(doc.Content)
		if len(chunks) == 0 {
			log.WithField("slug", doc.Slug).Warn("Standing doc produced no chunks — skipping")
			continue
		}

		embeddings := make([][]float32, 0, len(chunks))
		for i, chunk := range chunks {
			text := TruncateForEmbed(doc.Title+": "+chunk, 24000)
			mu.Lock()
			emb, err := ollama.Embed(text)
			mu.Unlock()
			if err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"slug":  doc.Slug,
					"chunk": i,
				}).Warn("Failed to embed chunk — skipping")
				continue
			}
			embeddings = append(embeddings, emb)
		}

		if len(embeddings) == 0 {
			log.WithField("slug", doc.Slug).Warn("All chunks failed to embed — skipping doc")
			continue
		}

		result = append(result, SlugChunks{
			Slug:   doc.Slug,
			Chunks: embeddings,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no standing docs produced embeddings")
	}

	return result, nil
}
