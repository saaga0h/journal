ALTER TABLE journal_entries ADD COLUMN until_timestamp TIMESTAMPTZ;

CREATE INDEX idx_journal_entries_time_range
    ON journal_entries(since_timestamp, until_timestamp)
    WHERE until_timestamp IS NOT NULL;
