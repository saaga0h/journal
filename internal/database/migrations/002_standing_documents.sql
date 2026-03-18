CREATE TABLE IF NOT EXISTS standing_documents (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768),
    version INT NOT NULL DEFAULT 1,
    source_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(slug, version)
);

CREATE INDEX idx_standing_documents_slug ON standing_documents(slug);
