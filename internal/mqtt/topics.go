package mqtt

const (
	// TopicEntriesIngest is where the concept extractor publishes entry data.
	TopicEntriesIngest = "journal/entries/ingest"

	// TopicEntriesCreated is published after an entry is persisted with embedding.
	TopicEntriesCreated = "journal/entries/created"

	// TopicStandingUpdated is published when a standing document is ingested or re-versioned.
	TopicStandingUpdated = "journal/standing/updated"

	// TopicConceptCreated is published when a project concept document is ingested.
	TopicConceptCreated = "journal/concepts/created"

	// TopicTrendCurrent is published by trend-detect with the current trend centroid.
	TopicTrendCurrent = "journal/trend/current"

	// TopicBriefTrigger triggers the morning brief assembly.
	TopicBriefTrigger = "journal/brief/trigger"

	// TopicBriefResult carries the brief result (one article or silence).
	TopicBriefResult = "journal/brief/result"

	// TopicMinervaQuery is published to Minerva to request relevant unread material.
	TopicMinervaQuery = "minerva/query/brief"

	// TopicMinervaResponse is where Minerva publishes brief query results back.
	TopicMinervaResponse = "journal/brief/minerva-response"
)
