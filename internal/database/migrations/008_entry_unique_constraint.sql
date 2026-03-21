CREATE UNIQUE INDEX IF NOT EXISTS idx_journal_entries_repo_since
    ON journal_entries (source, since_timestamp)
    WHERE since_timestamp IS NOT NULL;
