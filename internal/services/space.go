package services

import (
	"math"
	"time"

	"github.com/saaga0h/journal/internal/database"
)


// GravityProfile maps standing_slug -> GLF-weighted mean similarity score.
type GravityProfile map[string]float32

// ManifoldProfile maps standing_slug -> GLF+soul-speed-weighted mean proximity [0,1].
// Proximity = 1 - nearest_chunk_dist. Higher = entry semantics closer to that manifold.
type ManifoldProfile map[string]float32

// SlugChunks holds precomputed chunk embeddings for one standing doc.
type SlugChunks struct {
	Slug   string
	Chunks [][]float32
}

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

// ManifoldSpread measures how far entries are from the nearest chunk in embedding space.
// Returns the mean NearestChunkDistance across all entries. Returns 0 if no entries.
func ManifoldSpread(entries []database.ManifoldEntryPoint, chunkEmbeddings [][]float32) float32 {
	if len(entries) == 0 {
		return 0
	}
	var total float64
	for _, e := range entries {
		total += float64(NearestChunkDistance(e.Embedding.Slice(), chunkEmbeddings))
	}
	return float32(total / float64(len(entries)))
}

// ManifoldProximityProfile computes GLF+soul-speed-weighted mean proximity for each lateral
// standing doc manifold. Soul-speed similarity acts as a weight modifier: entries with higher
// soul-speed proximity contribute more strongly to each manifold's gravity.
//
// entries: recent entries with raw embeddings (from database.GetRecentEntryEmbeddings)
// slugChunks: per-slug chunk embeddings (computed via ComputeManifoldChunks)
// k, midpointDays: GLF recency parameters
func ManifoldProximityProfile(
	entries []database.ManifoldEntryPoint,
	slugChunks []SlugChunks,
	k, midpointDays float64,
) ManifoldProfile {
	now := time.Now()
	profile := make(ManifoldProfile, len(slugChunks))

	for _, sc := range slugChunks {
		if sc.Slug == SoulSpeedSlug || len(sc.Chunks) == 0 {
			continue
		}
		var weightedSum, totalWeight float64
		for _, e := range entries {
			ageDays := now.Sub(e.SinceTimestamp).Hours() / 24.0
			glfW := GLFWeight(ageDays, k, midpointDays)
			ssW := float64(soulSpeedSimilarity(e, slugChunks))
			// soul-speed in [0.5, 1.0] — never zero-weights an entry
			weight := glfW * (0.5 + 0.5*ssW)
			proximity := 1.0 - float64(NearestChunkDistance(e.Embedding.Slice(), sc.Chunks))
			weightedSum += proximity * weight
			totalWeight += weight
		}
		if totalWeight > 0 {
			profile[sc.Slug] = float32(weightedSum / totalWeight)
		}
	}
	return profile
}

// ManifoldSoulSpeed returns the GLF-weighted mean proximity to the soul-speed manifold.
// Replaces SoulSpeedProfile (which used association similarity, not manifold geometry).
func ManifoldSoulSpeed(
	entries []database.ManifoldEntryPoint,
	slugChunks []SlugChunks,
	k, midpointDays float64,
) float32 {
	now := time.Now()
	var weightedSum, totalWeight float64
	for _, sc := range slugChunks {
		if sc.Slug != SoulSpeedSlug || len(sc.Chunks) == 0 {
			continue
		}
		for _, e := range entries {
			ageDays := now.Sub(e.SinceTimestamp).Hours() / 24.0
			w := GLFWeight(ageDays, k, midpointDays)
			proximity := 1.0 - float64(NearestChunkDistance(e.Embedding.Slice(), sc.Chunks))
			weightedSum += proximity * w
			totalWeight += w
		}
		break
	}
	if totalWeight == 0 {
		return 0
	}
	return float32(weightedSum / totalWeight)
}

// soulSpeedSimilarity returns the entry's proximity to the soul-speed manifold [0,1].
// Returns 0 if no soul-speed slug chunks are available.
func soulSpeedSimilarity(e database.ManifoldEntryPoint, slugChunks []SlugChunks) float32 {
	for _, sc := range slugChunks {
		if sc.Slug == SoulSpeedSlug && len(sc.Chunks) > 0 {
			return 1.0 - NearestChunkDistance(e.Embedding.Slice(), sc.Chunks)
		}
	}
	return 0
}

// UnexpectedConceptsFromManifold returns the top N concepts from entries that have the
// lowest maximum proximity across all manifolds — entries that don't strongly belong to any
// known standing doc territory. These represent potential phase transitions or emergent thinking.
// GLF+soul-speed weighting is applied so recent, alive entries surface first.
func UnexpectedConceptsFromManifold(
	entries []database.ManifoldEntryPoint,
	slugChunks []SlugChunks,
	k, midpointDays float64,
	topN int,
) []string {
	now := time.Now()

	// Lateral chunks only (exclude soul-speed — it's a modifier, not a territory)
	var lateral []SlugChunks
	for _, sc := range slugChunks {
		if sc.Slug != SoulSpeedSlug && len(sc.Chunks) > 0 {
			lateral = append(lateral, sc)
		}
	}
	if len(lateral) == 0 {
		return nil
	}

	scores := make(map[string]float64)
	for _, e := range entries {
		// Max proximity to any manifold: low = entry doesn't fit anywhere
		var maxProx float64
		for _, sc := range lateral {
			p := 1.0 - float64(NearestChunkDistance(e.Embedding.Slice(), sc.Chunks))
			if p > maxProx {
				maxProx = p
			}
		}
		// Invert: high score = far from all manifolds
		unexpectedness := 1.0 - maxProx

		ageDays := now.Sub(e.SinceTimestamp).Hours() / 24.0
		glfW := GLFWeight(ageDays, k, midpointDays)
		ssW := float64(soulSpeedSimilarity(e, slugChunks))
		weight := glfW * (0.5 + 0.5*ssW)

		for _, concept := range e.Concepts {
			scores[concept] += unexpectedness * weight
		}
	}

	return topConcepts(scores, topN)
}

// TrendingConcepts returns the top N concepts by GLF-weighted frequency across entries.
// Each concept's score is the sum of GLF weights of entries that contain it.
func TrendingConcepts(points []database.EntrySpacePoint, k float64, midpointDays float64, topN int) []string {
	now := time.Now()
	scores := make(map[string]float64)

	for _, pt := range points {
		ageDays := now.Sub(pt.SinceTimestamp).Hours() / 24.0
		w := GLFWeight(ageDays, k, midpointDays)
		for _, concept := range pt.Concepts {
			scores[concept] += w
		}
	}

	return topConcepts(scores, topN)
}

// UnexpectedConcepts returns the top N concepts from entries that are most distant
// from the gravity profile centroid in standing-doc space. These represent thinking
// that doesn't fit the current gravitational pattern — potential phase transitions.
func UnexpectedConcepts(points []database.EntrySpacePoint, slugs []string, gravity GravityProfile, topN int) []string {
	lateralSlugs := LateralSlugs(slugs)

	// Score each concept by the spread distance of entries it appears in.
	// Concepts appearing in distant entries score higher.
	scores := make(map[string]float64)

	for _, pt := range points {
		var sumSq float64
		for _, slug := range lateralSlugs {
			centroidVal := float64(gravity[slug])
			pointVal := float64(pt.Coords[slug])
			diff := pointVal - centroidVal
			sumSq += diff * diff
		}
		dist := math.Sqrt(sumSq)
		for _, concept := range pt.Concepts {
			if dist > scores[concept] {
				scores[concept] = dist // keep the max distance an entry containing this concept has
			}
		}
	}

	// Exclude concepts that also appear in trending (they're not unexpected)
	trending := make(map[string]bool)
	for _, c := range TrendingConcepts(points, 0.3, 14.0, 20) {
		trending[c] = true
	}
	for c := range trending {
		delete(scores, c)
	}

	return topConcepts(scores, topN)
}

func topConcepts(scores map[string]float64, topN int) []string {
	type scored struct {
		concept string
		score   float64
	}
	ranked := make([]scored, 0, len(scores))
	for c, s := range scores {
		ranked = append(ranked, scored{c, s})
	}
	// Sort descending
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	if topN > len(ranked) {
		topN = len(ranked)
	}
	result := make([]string, topN)
	for i := 0; i < topN; i++ {
		result[i] = ranked[i].concept
	}
	return result
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
