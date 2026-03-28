package mqtt

import (
	"encoding/json"
	"time"
)

// Envelope is the common header carried in every message.
type Envelope struct {
	MessageID string    `json:"message_id"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// EntryIngest is published by the concept extractor to TopicEntriesIngest.
// Engineering and Theoretical are preserved as raw JSON from the extractor output.
type EntryIngest struct {
	Envelope
	Source       string          `json:"source"`
	SinceTimestamp   time.Time       `json:"since_timestamp"`
	UntilTimestamp   time.Time       `json:"until_timestamp,omitempty"`
	ExtractorVersion string          `json:"extractor_version"`
	Engineering      json.RawMessage `json:"engineering"`
	Theoretical      json.RawMessage `json:"theoretical,omitempty"`
	GitInput         string          `json:"git_input,omitempty"`
}

// EntryCreated is published by the entry-ingest service to TopicEntriesCreated.
type EntryCreated struct {
	Envelope
	EntryID    int64  `json:"entry_id"`
	Source string `json:"source"`
}

// StandingUpdated is published when a standing document is ingested or re-versioned.
type StandingUpdated struct {
	Envelope
	Slug    string `json:"slug"`
	Version int    `json:"version"`
}

// ConceptCreated is published when a project concept document is ingested.
type ConceptCreated struct {
	Envelope
	Slug    string `json:"slug"`
	Project string `json:"project"`
}

// TrendResult is published by trend-detect after computing the manifold proximity profile.
type TrendResult struct {
	Envelope
	ManifoldProfile    map[string]float32 `json:"manifold_profile"`     // slug -> GLF+soul-speed-weighted mean proximity [0,1]
	SoulSpeed          float32            `json:"soul_speed"`            // GLF-weighted proximity to soul-speed manifold
	TrendingConcepts   []string           `json:"trending_concepts"`     // top GLF-weighted concepts by frequency
	UnexpectedConcepts []string           `json:"unexpected_concepts"`   // concepts from entries distant from all manifolds
	EntryCount         int                `json:"entry_count"`
	WindowDays         int                `json:"window_days"`
	ComputedAt         time.Time          `json:"computed_at"`
	HumanSummary       string             `json:"human_summary"`         // pre-rendered readable description
}

// TrendException captures a candidate phase-transition signal.
// Retained for future use but not currently included in TrendResult.
type TrendException struct {
	EntryID               int64   `json:"entry_id,omitempty"`
	ActivatedStandingSlug string  `json:"activated_standing_slug"`
	CentroidDistance      float32 `json:"centroid_distance"`
	StandingSimilarity    float32 `json:"standing_similarity"`
}

// MinervaQuery is published to TopicMinervaQuery by brief-assemble.
// ManifoldProfile is always populated (human-readable scalar context).
// TrendEmbeddings and UnexpectedEmbeddings are populated when Ollama is available —
// Minerva uses these for ANN search when its corpus is embedded.
type MinervaQuery struct {
	SessionID            string             `json:"session_id"`
	ManifoldProfile      map[string]float32 `json:"manifold_profile"`
	TrendEmbeddings      [][]float32        `json:"trend_embeddings,omitempty"`
	UnexpectedEmbeddings [][]float32        `json:"unexpected_embeddings,omitempty"`
	SoulSpeed            float32            `json:"soul_speed"`
	TopK                 int                `json:"top_k"`
	ResponseTopic        string             `json:"response_topic"`
}

// BriefTrigger is published to TopicBriefTrigger to trigger morning brief assembly.
type BriefTrigger struct{ Envelope }

// BriefResult carries the morning brief output — one article or silence.
type BriefResult struct {
	Envelope
	SessionID     string    `json:"session_id"`
	ArticleURL    string    `json:"article_url,omitempty"`
	ArticleTitle  string    `json:"article_title,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	Silence       bool      `json:"silence"`
	SilenceReason string    `json:"silence_reason,omitempty"`
	TriggerTime   time.Time `json:"trigger_time"`
}
