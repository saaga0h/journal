DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'standing_documents' AND column_name = 'content_hash'
    ) THEN
        ALTER TABLE standing_documents ADD COLUMN content_hash TEXT;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS webdav_ingest_state (
    source_path TEXT PRIMARY KEY,
    content_hash TEXT NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
