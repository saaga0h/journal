package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetWebDAVState returns the stored content hash for a previously ingested WebDAV file.
// Returns (hash, true, nil) if found, ("", false, nil) if not found.
func GetWebDAVState(pool *pgxpool.Pool, sourcePath string) (string, bool, error) {
	ctx := context.Background()
	var hash string
	err := pool.QueryRow(ctx,
		`SELECT content_hash FROM webdav_ingest_state WHERE source_path = $1`,
		sourcePath,
	).Scan(&hash)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to get webdav state for %s: %w", sourcePath, err)
	}
	return hash, true, nil
}

// UpsertWebDAVState records or updates the content hash for an ingested WebDAV file.
func UpsertWebDAVState(pool *pgxpool.Pool, sourcePath, contentHash string) error {
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO webdav_ingest_state (source_path, content_hash, ingested_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (source_path) DO UPDATE SET content_hash = $2, ingested_at = NOW()`,
		sourcePath, contentHash,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert webdav state for %s: %w", sourcePath, err)
	}
	return nil
}
