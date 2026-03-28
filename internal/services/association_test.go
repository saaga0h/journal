package services

import (
	"math"
	"testing"
)

// --- CosineSimilarity ---

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	got := CosineSimilarity(a, a)
	if math.Abs(float64(got)-1.0) > 1e-5 {
		t.Errorf("want 1.0, got %.6f", got)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	got := CosineSimilarity(a, b)
	if math.Abs(float64(got)+1.0) > 1e-5 {
		t.Errorf("want -1.0, got %.6f", got)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	got := CosineSimilarity(a, b)
	if math.Abs(float64(got)) > 1e-5 {
		t.Errorf("want 0.0, got %.6f", got)
	}
}

func TestCosineSimilarity_DimensionMismatch(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("want 0 for dimension mismatch, got %.6f", got)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0}
	b := []float32{1, 1}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("want 0 for zero vector, got %.6f", got)
	}
}

func TestCosineSimilarity_FortyFiveDegrees(t *testing.T) {
	// [1/√2, 1/√2] is at 45° from [1, 0]; cos(45°) ≈ 0.7071
	inv := float32(1.0 / math.Sqrt(2))
	a := []float32{inv, inv}
	b := []float32{1, 0}
	got := CosineSimilarity(a, b)
	want := float32(math.Sqrt(2) / 2)
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Errorf("want %.6f (cos 45°), got %.6f", want, got)
	}
}

// --- NearestChunkDistance ---

func TestNearestChunkDistance_EmptyChunks(t *testing.T) {
	entry := []float32{1, 0, 0}
	got := NearestChunkDistance(entry, nil)
	if got != 1.0 {
		t.Errorf("want 1.0 for empty chunks, got %.6f", got)
	}
}

func TestNearestChunkDistance_SingleChunk_Identical(t *testing.T) {
	entry := []float32{1, 0, 0}
	chunks := [][]float32{{1, 0, 0}}
	got := NearestChunkDistance(entry, chunks)
	if math.Abs(float64(got)) > 1e-5 {
		t.Errorf("want 0.0 for identical chunk, got %.6f", got)
	}
}

func TestNearestChunkDistance_SingleChunk_Orthogonal(t *testing.T) {
	entry := []float32{1, 0}
	chunks := [][]float32{{0, 1}}
	got := NearestChunkDistance(entry, chunks)
	if math.Abs(float64(got)-1.0) > 1e-5 {
		t.Errorf("want 1.0 for orthogonal chunk, got %.6f", got)
	}
}

func TestNearestChunkDistance_MultipleChunks_PicksNearest(t *testing.T) {
	entry := []float32{1, 0, 0}
	chunks := [][]float32{
		{0, 1, 0}, // orthogonal
		{1, 0, 0}, // identical — nearest
		{0, 0, 1}, // orthogonal
	}
	got := NearestChunkDistance(entry, chunks)
	if math.Abs(float64(got)) > 1e-5 {
		t.Errorf("want 0.0 (nearest chunk is identical), got %.6f", got)
	}
}

func TestNearestChunkDistance_MultipleChunks_PartialMatch(t *testing.T) {
	// entry at 45° to nearest chunk: distance = 1 - cos(45°) ≈ 0.2929
	inv := float32(1.0 / math.Sqrt(2))
	entry := []float32{inv, inv}
	chunks := [][]float32{
		{1, 0}, // 45° away
		{0, 1}, // also 45° away
	}
	got := NearestChunkDistance(entry, chunks)
	want := float32(1.0 - math.Sqrt(2)/2)
	if math.Abs(float64(got-want)) > 1e-5 {
		t.Errorf("want %.6f (1 - cos45°), got %.6f", want, got)
	}
}
