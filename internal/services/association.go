package services

import (
	"math"
	"sort"

	"github.com/saaga0h/journal/internal/database"
)

// AssociationResult records that an entry is similar to a standing document.
type AssociationResult struct {
	StandingSlug string
	Similarity   float32
}

// ComputeStandingAssociations computes cosine similarity between an entry embedding
// and each standing document embedding. Returns all pairs above the threshold,
// sorted by similarity descending.
func ComputeStandingAssociations(entryEmbedding []float32, standings []database.StandingDocumentEmbedding, threshold float32) []AssociationResult {
	var results []AssociationResult

	for _, sd := range standings {
		sim := cosineSimilarity(entryEmbedding, sd.Embedding.Slice())
		if sim >= threshold {
			results = append(results, AssociationResult{
				StandingSlug: sd.Slug,
				Similarity:   sim,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	return results
}

// NearestChunkDistance returns 1 - max cosine similarity between entryEmbedding
// and any chunk in chunks. Returns 1.0 if chunks is empty.
// Distance 0 = entry is identical to a chunk; 1 = completely unrelated.
func NearestChunkDistance(entryEmbedding []float32, chunks [][]float32) float32 {
	if len(chunks) == 0 {
		return 1.0
	}
	var maxSim float32
	for _, chunk := range chunks {
		if sim := cosineSimilarity(entryEmbedding, chunk); sim > maxSim {
			maxSim = sim
		}
	}
	return 1.0 - maxSim
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}

	return float32(dot / denom)
}
