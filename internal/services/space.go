package services

import (
	"math"
	"time"

	"github.com/saaga0h/journal/internal/database"
)

// GravityProfile maps standing_slug -> GLF-weighted mean similarity score.
type GravityProfile map[string]float32

// SoulSpeedSlug is the standing document slug treated as a perpendicular axis.
const SoulSpeedSlug = "soul-speed"

// GLFWeightedGravityProfile computes the recency-weighted mean similarity for each
// lateral standing doc dimension (all slugs except soul-speed).
// slugs is the full list of standing doc slugs (soul-speed will be filtered out).
// k and midpointDays are GLF parameters.
func GLFWeightedGravityProfile(points []database.EntrySpacePoint, slugs []string, k, midpointDays float64) GravityProfile {
	now := time.Now()
	profile := make(GravityProfile, len(slugs))

	lateralSlugs := LateralSlugs(slugs)

	for _, slug := range lateralSlugs {
		var weightedSum, totalWeight float64
		for _, pt := range points {
			sim, ok := pt.Coords[slug]
			if !ok {
				continue
			}
			ageDays := now.Sub(pt.SinceTimestamp).Hours() / 24.0
			w := GLFWeight(ageDays, k, midpointDays)
			weightedSum += float64(sim) * w
			totalWeight += w
		}
		if totalWeight > 0 {
			profile[slug] = float32(weightedSum / totalWeight)
		}
	}

	return profile
}

// SoulSpeedProfile computes the GLF-weighted mean similarity for the soul-speed dimension.
func SoulSpeedProfile(points []database.EntrySpacePoint, k, midpointDays float64) float32 {
	now := time.Now()
	var weightedSum, totalWeight float64

	for _, pt := range points {
		sim, ok := pt.Coords[SoulSpeedSlug]
		if !ok {
			continue
		}
		ageDays := now.Sub(pt.SinceTimestamp).Hours() / 24.0
		w := GLFWeight(ageDays, k, midpointDays)
		weightedSum += float64(sim) * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0
	}
	return float32(weightedSum / totalWeight)
}

// ClusterSpread measures how dispersed entries are around the gravity profile centroid
// in lateral standing-doc space. Returns mean Euclidean distance from centroid.
// Low = tight cluster, high = dispersed thinking.
func ClusterSpread(points []database.EntrySpacePoint, slugs []string, gravity GravityProfile) float32 {
	lateralSlugs := LateralSlugs(slugs)

	if len(points) == 0 || len(lateralSlugs) == 0 {
		return 0
	}

	var totalDist float64
	for _, pt := range points {
		var sumSq float64
		for _, slug := range lateralSlugs {
			centroidVal := float64(gravity[slug])
			pointVal := float64(pt.Coords[slug]) // 0 if missing
			diff := pointVal - centroidVal
			sumSq += diff * diff
		}
		totalDist += math.Sqrt(sumSq)
	}

	return float32(totalDist / float64(len(points)))
}

// LateralSlugs returns all slugs from the provided list except soul-speed.
func LateralSlugs(allSlugs []string) []string {
	lateral := make([]string, 0, len(allSlugs))
	for _, s := range allSlugs {
		if s != SoulSpeedSlug {
			lateral = append(lateral, s)
		}
	}
	return lateral
}
