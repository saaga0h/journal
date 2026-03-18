package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saaga0h/journal/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Connect creates a connection pool to Postgres.
func Connect(cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations applies all pending SQL migrations in filename order.
// Migrations are tracked in the schema_migrations table.
func RunMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Ensure schema_migrations table exists (bootstrap)
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Read all migration files
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	// Sort by filename
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := entry.Name()

		// Check if already applied
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		// Read and execute migration
		content, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", version, err)
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}

		// Record migration
		_, err = pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			version,
		)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}
	}

	return nil
}
