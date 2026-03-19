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
	Repository       string          `json:"repository"`
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
	Repository string `json:"repository"`
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

// TrendResult is published by trend-detect after computing the current trend centroid.
type TrendResult struct {
	Envelope
	TrendVector []float32       `json:"trend_vector"`
	EntryCount  int             `json:"entry_count"`
	WindowDays  int             `json:"window_days"`
	Exceptions  []TrendException `json:"exceptions,omitempty"`
	ComputedAt  time.Time       `json:"computed_at"`
}

// TrendException captures a candidate phase-transition signal: an entry distant from
// the current trend centroid but close to a standing document not recently activated.
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
