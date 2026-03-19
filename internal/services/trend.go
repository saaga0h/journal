package services

import (
	"fmt"
	"math"
)

// GLFWeight computes the Generalized Logistic Function recency weight for an entry.
// ageDays is how many days ago the entry's window started.
// k controls steepness of the sigmoid falloff (higher = sharper).
// midpointDays is the age at which weight = 0.5.
//
// Result: entries younger than midpoint get weight > 0.5, older entries < 0.5.
// With defaults k=0.3, midpoint=14: entries < 7 days old get ~0.9-1.0 weight;
// entries 3-4 weeks old get ~0.1-0.3 weight.
func GLFWeight(ageDays, k, midpointDays float64) float64 {
	return 1.0 / (1.0 + math.Exp(k*(ageDays-midpointDays)))
}

// WeightedCentroid computes the weighted mean of embedding vectors.
// embeddings and weights must have the same length and length > 0.
// Returns a normalised centroid vector.
func WeightedCentroid(embeddings [][]float32, weights []float64) ([]float32, error) {
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings provided")
	}
	if len(embeddings) != len(weights) {
		return nil, fmt.Errorf("embeddings and weights length mismatch: %d vs %d", len(embeddings), len(weights))
	}

	dim := len(embeddings[0])
	centroid := make([]float64, dim)
	totalWeight := 0.0

	for i, emb := range embeddings {
		if len(emb) != dim {
			return nil, fmt.Errorf("embedding %d has dimension %d, expected %d", i, len(emb), dim)
		}
		w := weights[i]
		totalWeight += w
		for j, v := range emb {
			centroid[j] += float64(v) * w
		}
	}

	if totalWeight == 0 {
		return nil, fmt.Errorf("total weight is zero")
	}

	result := make([]float32, dim)
	for j := range centroid {
		result[j] = float32(centroid[j] / totalWeight)
	}

	return result, nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}

	magA = math.Sqrt(magA)
	magB = math.Sqrt(magB)

	if magA == 0 || magB == 0 {
		return 0
	}

	return float32(dot / (magA * magB))
}
