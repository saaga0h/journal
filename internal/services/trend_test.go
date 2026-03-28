package services

import (
	"math"
	"testing"
)

// --- GLFWeight ---

func TestGLFWeight_AtMidpoint(t *testing.T) {
	// At ageDays == midpointDays: 1/(1+e^0) = 0.5 exactly
	got := GLFWeight(14, 0.3, 14)
	if got != 0.5 {
		t.Errorf("want 0.5 at midpoint, got %.10f", got)
	}
}

func TestGLFWeight_YoungEntry(t *testing.T) {
	// age 0, k=0.3, midpoint=14: 1/(1+e^(-4.2)) ≈ 0.985
	got := GLFWeight(0, 0.3, 14)
	if got < 0.9 {
		t.Errorf("want > 0.9 for very young entry, got %.6f", got)
	}
}

func TestGLFWeight_OldEntry(t *testing.T) {
	// age 28, k=0.3, midpoint=14: 1/(1+e^(4.2)) ≈ 0.015
	got := GLFWeight(28, 0.3, 14)
	if got > 0.1 {
		t.Errorf("want < 0.1 for old entry, got %.6f", got)
	}
}

func TestGLFWeight_DecayMonotonic(t *testing.T) {
	ages := []float64{0, 7, 14, 21, 28}
	prev := GLFWeight(ages[0], 0.3, 14)
	for _, age := range ages[1:] {
		w := GLFWeight(age, 0.3, 14)
		if w >= prev {
			t.Errorf("expected strictly decreasing weights; age %.0f weight %.6f >= prev %.6f", age, w, prev)
		}
		prev = w
	}
}

// --- WeightedCentroid ---

func TestWeightedCentroid_SingleVector(t *testing.T) {
	embs := [][]float32{{0.1, 0.5, 0.9}}
	got, err := WeightedCentroid(embs, []float64{1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range embs[0] {
		if math.Abs(float64(got[i]-v)) > 1e-5 {
			t.Errorf("dim %d: want %.6f, got %.6f", i, v, got[i])
		}
	}
}

func TestWeightedCentroid_EqualWeights(t *testing.T) {
	embs := [][]float32{{1, 0, 0}, {0, 1, 0}}
	got, err := WeightedCentroid(embs, []float64{1.0, 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float32{0.5, 0.5, 0}
	for i, w := range want {
		if math.Abs(float64(got[i]-w)) > 1e-5 {
			t.Errorf("dim %d: want %.6f, got %.6f", i, w, got[i])
		}
	}
}

func TestWeightedCentroid_UnequalWeights(t *testing.T) {
	embs := [][]float32{{1, 0, 0}, {0, 1, 0}}
	got, err := WeightedCentroid(embs, []float64{3.0, 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float32{0.75, 0.25, 0}
	for i, w := range want {
		if math.Abs(float64(got[i]-w)) > 1e-5 {
			t.Errorf("dim %d: want %.6f, got %.6f", i, w, got[i])
		}
	}
}

func TestWeightedCentroid_ZeroTotalWeight(t *testing.T) {
	embs := [][]float32{{1, 0}, {0, 1}}
	_, err := WeightedCentroid(embs, []float64{0, 0})
	if err == nil {
		t.Error("want error for zero total weight, got nil")
	}
}

func TestWeightedCentroid_Empty(t *testing.T) {
	_, err := WeightedCentroid(nil, nil)
	if err == nil {
		t.Error("want error for empty embeddings, got nil")
	}
}

func TestWeightedCentroid_DimensionMismatch(t *testing.T) {
	embs := [][]float32{{1, 0}, {1, 0, 0}}
	_, err := WeightedCentroid(embs, []float64{1.0, 1.0})
	if err == nil {
		t.Error("want error for dimension mismatch, got nil")
	}
}
