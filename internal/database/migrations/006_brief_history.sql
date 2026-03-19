CREATE TABLE IF NOT EXISTS brief_history (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE,
    triggered_at TIMESTAMPTZ NOT NULL,
    trend_snapshot JSONB,
    article_url TEXT,
    article_title TEXT,
    relevance_score REAL,
    silence_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS brief_feedback (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES brief_history(session_id),
    action TEXT NOT NULL CHECK (action IN ('read', 'skip')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
