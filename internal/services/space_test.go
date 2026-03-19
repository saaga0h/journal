package services

import (
	"testing"
	"time"

	"github.com/saaga0h/journal/internal/database"
)

func TestGLFWeightedGravityProfile(t *testing.T) {
	now := time.Now()
	points := []database.EntrySpacePoint{
		{
			EntryID: 1, SinceTimestamp: now.AddDate(0, 0, -1),
			Coords: map[string]float32{
				"distributed-patterns": 0.7, "universe-design": 0.5, "soul-speed": 0.6,
			},
		},
		{
			EntryID: 2, SinceTimestamp: now.AddDate(0, 0, -20),
			Coords: map[string]float32{
				"distributed-patterns": 0.3, "universe-design": 0.8, "soul-speed": 0.4,
			},
		},
	}
	slugs := []string{"distributed-patterns", "universe-design", "soul-speed"}

	profile := GLFWeightedGravityProfile(points, slugs, 0.3, 14.0)

	// Soul-speed must be excluded from gravity profile
	if _, hasSoulSpeed := profile["soul-speed"]; hasSoulSpeed {
		t.Error("gravity profile should not contain soul-speed")
	}

	// Recent entry (1 day old) should dominate over old entry (20 days old)
	// So distributed-patterns should be closer to 0.7 than 0.3
	dp := profile["distributed-patterns"]
	if dp < 0.5 {
		t.Errorf("expected distributed-patterns to be pulled toward recent entry (0.7), got %.3f", dp)
	}
}

func TestSoulSpeedProfile(t *testing.T) {
	now := time.Now()
	points := []database.EntrySpacePoint{
		{EntryID: 1, SinceTimestamp: now, Coords: map[string]float32{"soul-speed": 0.8}},
		{EntryID: 2, SinceTimestamp: now.AddDate(0, 0, -30), Coords: map[string]float32{"soul-speed": 0.2}},
	}

	ss := SoulSpeedProfile(points, 0.3, 14.0)
	if ss < 0.5 {
		t.Errorf("expected soul-speed to be dominated by recent entry (0.8), got %.3f", ss)
	}
}

func TestSoulSpeedProfileEmpty(t *testing.T) {
	ss := SoulSpeedProfile(nil, 0.3, 14.0)
	if ss != 0 {
		t.Errorf("expected 0 for empty points, got %.3f", ss)
	}
}

func TestClusterSpread(t *testing.T) {
	// All entries at the same coords -> spread = 0
	coords := map[string]float32{"distributed-patterns": 0.5, "universe-design": 0.6}
	points := []database.EntrySpacePoint{
		{EntryID: 1, Coords: coords},
		{EntryID: 2, Coords: coords},
	}
	gravity := GravityProfile{"distributed-patterns": 0.5, "universe-design": 0.6}
	slugs := []string{"distributed-patterns", "universe-design"}

	spread := ClusterSpread(points, slugs, gravity)
	if spread != 0 {
		t.Errorf("expected 0 spread for identical points, got %.3f", spread)
	}
}

func TestClusterSpreadNonZero(t *testing.T) {
	points := []database.EntrySpacePoint{
		{EntryID: 1, Coords: map[string]float32{"a": 0.8, "b": 0.2}},
		{EntryID: 2, Coords: map[string]float32{"a": 0.2, "b": 0.8}},
	}
	gravity := GravityProfile{"a": 0.5, "b": 0.5}
	slugs := []string{"a", "b"}

	spread := ClusterSpread(points, slugs, gravity)
	if spread <= 0 {
		t.Error("expected positive spread for dispersed points")
	}
}

func TestLateralSlugs(t *testing.T) {
	all := []string{"a", "soul-speed", "b"}
	lateral := LateralSlugs(all)
	if len(lateral) != 2 {
		t.Errorf("expected 2 lateral slugs, got %d", len(lateral))
	}
	for _, s := range lateral {
		if s == SoulSpeedSlug {
			t.Error("soul-speed should not appear in lateral slugs")
		}
	}
}
