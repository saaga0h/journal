package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// StandingDocument represents a versioned standing document stored in the database.
type StandingDocument struct {
	ID         int64
	Slug       string
	Title      string
	Content    string
	Embedding  pgvector.Vector
	Version    int
	SourcePath string
	CreatedAt  time.Time
}

// StandingDocumentEmbedding holds a slug and its current embedding for association queries.
type StandingDocumentEmbedding struct {
	Slug      string
	Title     string
	Embedding pgvector.Vector
}

// InsertStandingDocument stores a standing document with auto-incrementing version.
// Returns the database ID and the assigned version number.
// contentHash is a SHA256 hex digest of the content; pass empty string to leave null.
func InsertStandingDocument(pool *pgxpool.Pool, slug, title, content string, embedding pgvector.Vector, sourcePath string, contentHash string) (int64, int, error) {
	ctx := context.Background()

	// Determine next version
	var nextVersion int
	err := pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(version), 0) + 1 FROM standing_documents WHERE slug = $1",
		slug,
	).Scan(&nextVersion)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to determine next version for slug %s: %w", slug, err)
	}

	var hashArg interface{}
	if contentHash != "" {
		hashArg = contentHash
	}

	var id int64
	err = pool.QueryRow(ctx,
		`INSERT INTO standing_documents (slug, title, content, embedding, version, source_path, content_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		slug, title, content, embedding, nextVersion, sourcePath, hashArg,
	).Scan(&id)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to insert standing document %s v%d: %w", slug, nextVersion, err)
	}

	return id, nextVersion, nil
}

// GetStandingDocumentHash returns the content_hash of the current version of a standing document.
// Returns empty string if the document doesn't exist or has no hash stored.
func GetStandingDocumentHash(pool *pgxpool.Pool, slug string) (string, error) {
	ctx := context.Background()
	var hash *string
	err := pool.QueryRow(ctx,
		`SELECT content_hash FROM standing_documents WHERE slug = $1 ORDER BY version DESC LIMIT 1`,
		slug,
	).Scan(&hash)
	if err != nil {
		// No rows = document doesn't exist yet
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("failed to get content hash for slug %s: %w", slug, err)
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

// GetCurrentStandingDocument returns the latest version of a standing document by slug.
func GetCurrentStandingDocument(pool *pgxpool.Pool, slug string) (*StandingDocument, error) {
	ctx := context.Background()
	doc := &StandingDocument{}

	var embedding *pgvector.Vector
	err := pool.QueryRow(ctx,
		`SELECT id, slug, title, content, embedding, version, source_path, created_at
		 FROM standing_documents
		 WHERE slug = $1
		 ORDER BY version DESC
		 LIMIT 1`,
		slug,
	).Scan(&doc.ID, &doc.Slug, &doc.Title, &doc.Content, &embedding, &doc.Version, &doc.SourcePath, &doc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get standing document %s: %w", slug, err)
	}
	if embedding != nil {
		doc.Embedding = *embedding
	}

	return doc, nil
}

// GetStandingDocumentVersion returns a specific version of a standing document.
func GetStandingDocumentVersion(pool *pgxpool.Pool, slug string, version int) (*StandingDocument, error) {
	ctx := context.Background()
	doc := &StandingDocument{}

	var embedding *pgvector.Vector
	err := pool.QueryRow(ctx,
		`SELECT id, slug, title, content, embedding, version, source_path, created_at
		 FROM standing_documents
		 WHERE slug = $1 AND version = $2`,
		slug, version,
	).Scan(&doc.ID, &doc.Slug, &doc.Title, &doc.Content, &embedding, &doc.Version, &doc.SourcePath, &doc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get standing document %s v%d: %w", slug, version, err)
	}
	if embedding != nil {
		doc.Embedding = *embedding
	}

	return doc, nil
}

// ListCurrentStandingDocuments returns the latest version of each standing document.
func ListCurrentStandingDocuments(pool *pgxpool.Pool) ([]StandingDocument, error) {
	ctx := context.Background()

	rows, err := pool.Query(ctx,
		`SELECT sd.id, sd.slug, sd.title, sd.content, sd.embedding, sd.version, sd.source_path, sd.created_at
		 FROM standing_documents sd
		 WHERE sd.version = (SELECT MAX(version) FROM standing_documents sd2 WHERE sd2.slug = sd.slug)
		 ORDER BY sd.slug`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list standing documents: %w", err)
	}
	defer rows.Close()

	var docs []StandingDocument
	for rows.Next() {
		var doc StandingDocument
		var embedding *pgvector.Vector
		if err := rows.Scan(&doc.ID, &doc.Slug, &doc.Title, &doc.Content, &embedding, &doc.Version, &doc.SourcePath, &doc.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan standing document: %w", err)
		}
		if embedding != nil {
			doc.Embedding = *embedding
		}
		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// GetAllCurrentEmbeddings returns slug and embedding for all current standing documents.
// Used for computing entry-standing associations.
func GetAllCurrentEmbeddings(pool *pgxpool.Pool) ([]StandingDocumentEmbedding, error) {
	ctx := context.Background()

	rows, err := pool.Query(ctx,
		`SELECT sd.slug, sd.title, sd.embedding
		 FROM standing_documents sd
		 WHERE sd.embedding IS NOT NULL
		   AND sd.version = (SELECT MAX(version) FROM standing_documents sd2 WHERE sd2.slug = sd.slug)
		 ORDER BY sd.slug`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get standing document embeddings: %w", err)
	}
	defer rows.Close()

	var embeddings []StandingDocumentEmbedding
	for rows.Next() {
		var e StandingDocumentEmbedding
		if err := rows.Scan(&e.Slug, &e.Title, &e.Embedding); err != nil {
			return nil, fmt.Errorf("failed to scan standing document embedding: %w", err)
		}
		embeddings = append(embeddings, e)
	}

	return embeddings, rows.Err()
}
