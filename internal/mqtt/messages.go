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

// TrendResult is published by trend-detect after computing the gravity profile
// in standing-document space.
type TrendResult struct {
	Envelope
	GravityProfile     map[string]float32 `json:"gravity_profile"`     // slug -> weighted mean similarity
	SoulSpeed          float32            `json:"soul_speed"`           // Soul Speed axis score
	ClusterSpread      float32            `json:"cluster_spread"`       // tightness of the cluster
	TrendingConcepts   []string           `json:"trending_concepts"`    // top GLF-weighted concepts by frequency
	UnexpectedConcepts []string           `json:"unexpected_concepts"`  // concepts from spatially distant entries
	EntryCount         int                `json:"entry_count"`
	WindowDays         int                `json:"window_days"`
	ComputedAt         time.Time          `json:"computed_at"`
	HumanSummary       string             `json:"human_summary"` // pre-rendered readable description
}

// TrendException captures a candidate phase-transition signal.
// Retained for future use but not currently included in TrendResult.
type TrendException struct {
	EntryID               int64   `json:"entry_id,omitempty"`
	ActivatedStandingSlug string  `json:"activated_standing_slug"`
	CentroidDistance      float32 `json:"centroid_distance"`
	StandingSimilarity    float32 `json:"standing_similarity"`
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
