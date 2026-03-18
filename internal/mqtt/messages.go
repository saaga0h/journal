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
	ExtractorVersion string          `json:"extractor_version"`
	Engineering      json.RawMessage `json:"engineering"`
	Theoretical      json.RawMessage `json:"theoretical,omitempty"`
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
