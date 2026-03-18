CREATE TABLE IF NOT EXISTS journal_entries (
    id BIGSERIAL PRIMARY KEY,
    repository TEXT NOT NULL,
    since_timestamp TIMESTAMPTZ,
    extractor_version TEXT,
    engineering JSONB,
    theoretical JSONB,
    summary TEXT,
    concepts TEXT[],
    theoretical_territory TEXT[],
    annotation TEXT,
    embedding vector(768),
    raw_output JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_entries_repository ON journal_entries(repository);
CREATE INDEX idx_journal_entries_created_at ON journal_entries(created_at DESC);

CREATE TABLE IF NOT EXISTS entry_standing_associations (
    entry_id BIGINT NOT NULL REFERENCES journal_entries(id),
    standing_slug TEXT NOT NULL,
    similarity REAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entry_id, standing_slug)
);
