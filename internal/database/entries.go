package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// JournalEntry represents a timestamped record of concept extractor output.
type JournalEntry struct {
	ID                   int64
	Repository           string
	SinceTimestamp       time.Time
	UntilTimestamp       *time.Time
	ExtractorVersion     string
	Engineering          json.RawMessage
	Theoretical          json.RawMessage
	Summary              string
	Concepts             []string
	TheoreticalTerritory []string
	Annotation           string
	Embedding            pgvector.Vector
	GitInput             string
	RawOutput            json.RawMessage
	CreatedAt            time.Time
}

// EntryStandingAssociation records the cosine similarity between an entry and a standing document.
type EntryStandingAssociation struct {
	StandingSlug string
	Similarity   float32
}

// EntrySpacePoint represents a journal entry positioned in standing-document space.
// Coords maps standing_slug -> similarity score. No raw embedding needed.
type EntrySpacePoint struct {
	EntryID        int64
	Repository     string
	SinceTimestamp time.Time
	CreatedAt      time.Time
	Coords         map[string]float32 // slug -> similarity score
}

// ListEntriesOpts controls filtering for ListEntries queries.
type ListEntriesOpts struct {
	Repository string
	Since      time.Time
	Until      time.Time
	Limit      int
}

// InsertEntry stores a journal entry and returns its database ID.
func InsertEntry(pool *pgxpool.Pool, entry *JournalEntry) (int64, error) {
	ctx := context.Background()

	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO journal_entries
		 (repository, since_timestamp, until_timestamp, extractor_version, engineering, theoretical,
		  summary, concepts, theoretical_territory, annotation, embedding, git_input, raw_output)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id`,
		entry.Repository, entry.SinceTimestamp, entry.UntilTimestamp, entry.ExtractorVersion,
		entry.Engineering, entry.Theoretical,
		entry.Summary, entry.Concepts, entry.TheoreticalTerritory,
		entry.Annotation, entry.Embedding, entry.GitInput, entry.RawOutput,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert journal entry: %w", err)
	}

	return id, nil
}

// GetEntry returns a journal entry by ID.
func GetEntry(pool *pgxpool.Pool, id int64) (*JournalEntry, error) {
	ctx := context.Background()
	e := &JournalEntry{}
	var gitInput *string

	err := pool.QueryRow(ctx,
		`SELECT id, repository, since_timestamp, until_timestamp, extractor_version,
		        engineering, theoretical, summary, concepts, theoretical_territory,
		        annotation, embedding, git_input, raw_output, created_at
		 FROM journal_entries
		 WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.Repository, &e.SinceTimestamp, &e.UntilTimestamp, &e.ExtractorVersion,
		&e.Engineering, &e.Theoretical, &e.Summary, &e.Concepts, &e.TheoreticalTerritory,
		&e.Annotation, &e.Embedding, &gitInput, &e.RawOutput, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get journal entry %d: %w", id, err)
	}
	if gitInput != nil {
		e.GitInput = *gitInput
	}

	return e, nil
}

// ListEntries returns journal entries matching the given filters.
// Default limit is 100 if not specified.
func ListEntries(pool *pgxpool.Pool, opts ListEntriesOpts) ([]JournalEntry, error) {
	ctx := context.Background()

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, repository, since_timestamp, until_timestamp, extractor_version,
	                 engineering, theoretical, summary, concepts, theoretical_territory,
	                 annotation, embedding, git_input, raw_output, created_at
	          FROM journal_entries WHERE 1=1`
	args := []any{}
	argN := 1

	if opts.Repository != "" {
		query += fmt.Sprintf(" AND repository = $%d", argN)
		args = append(args, opts.Repository)
		argN++
	}
	if !opts.Since.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argN)
		args = append(args, opts.Since)
		argN++
	}
	if !opts.Until.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", argN)
		args = append(args, opts.Until)
		argN++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list journal entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// UpdateAnnotation sets the annotation text for a journal entry.
func UpdateAnnotation(pool *pgxpool.Pool, id int64, annotation string) error {
	ctx := context.Background()

	tag, err := pool.Exec(ctx,
		"UPDATE journal_entries SET annotation = $1 WHERE id = $2",
		annotation, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update annotation for entry %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("entry %d not found", id)
	}

	return nil
}

// UpdateEmbedding sets the embedding vector for a journal entry.
func UpdateEmbedding(pool *pgxpool.Pool, id int64, embedding pgvector.Vector) error {
	ctx := context.Background()

	tag, err := pool.Exec(ctx,
		"UPDATE journal_entries SET embedding = $1 WHERE id = $2",
		embedding, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update embedding for entry %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("entry %d not found", id)
	}

	return nil
}

// GetEntriesWithoutEmbedding returns entries that have null embeddings.
func GetEntriesWithoutEmbedding(pool *pgxpool.Pool) ([]JournalEntry, error) {
	ctx := context.Background()

	rows, err := pool.Query(ctx,
		`SELECT id, repository, since_timestamp, until_timestamp, extractor_version,
		        engineering, theoretical, summary, concepts, theoretical_territory,
		        annotation, embedding, git_input, raw_output, created_at
		 FROM journal_entries
		 WHERE embedding IS NULL
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries without embedding: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// InsertEntryStandingAssociation records a similarity between an entry and a standing document.
func InsertEntryStandingAssociation(pool *pgxpool.Pool, entryID int64, standingSlug string, similarity float32) error {
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO entry_standing_associations (entry_id, standing_slug, similarity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (entry_id, standing_slug) DO UPDATE SET similarity = EXCLUDED.similarity`,
		entryID, standingSlug, similarity,
	)
	if err != nil {
		return fmt.Errorf("failed to insert association entry=%d standing=%s: %w", entryID, standingSlug, err)
	}

	return nil
}

// GetEntryAssociations returns all standing document associations for a given entry.
func GetEntryAssociations(pool *pgxpool.Pool, entryID int64) ([]EntryStandingAssociation, error) {
	ctx := context.Background()

	rows, err := pool.Query(ctx,
		`SELECT standing_slug, similarity
		 FROM entry_standing_associations
		 WHERE entry_id = $1
		 ORDER BY similarity DESC`,
		entryID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get associations for entry %d: %w", entryID, err)
	}
	defer rows.Close()

	var assocs []EntryStandingAssociation
	for rows.Next() {
		var a EntryStandingAssociation
		if err := rows.Scan(&a.StandingSlug, &a.Similarity); err != nil {
			return nil, fmt.Errorf("failed to scan association: %w", err)
		}
		assocs = append(assocs, a)
	}

	return assocs, rows.Err()
}

// GetEntriesByStandingSlug returns entries associated with a specific standing document.
func GetEntriesByStandingSlug(pool *pgxpool.Pool, slug string, limit int) ([]JournalEntry, error) {
	ctx := context.Background()

	if limit <= 0 {
		limit = 100
	}

	rows, err := pool.Query(ctx,
		`SELECT je.id, je.repository, je.since_timestamp, je.until_timestamp, je.extractor_version,
		        je.engineering, je.theoretical, je.summary, je.concepts, je.theoretical_territory,
		        je.annotation, je.embedding, je.git_input, je.raw_output, je.created_at
		 FROM journal_entries je
		 JOIN entry_standing_associations esa ON esa.entry_id = je.id
		 WHERE esa.standing_slug = $1
		 ORDER BY esa.similarity DESC
		 LIMIT $2`,
		slug, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries by standing slug %s: %w", slug, err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// GetRecentEntriesWithEmbeddings returns entries with embeddings ingested within the last windowDays days.
// Uses created_at (ingestion time) not since_timestamp (git window open) for recency — an entry
// covering 360 days of git history is still "recent" if it was ingested today.
// Results are ordered by since_timestamp descending (most recent git window first).
func GetRecentEntriesWithEmbeddings(pool *pgxpool.Pool, windowDays int) ([]JournalEntry, error) {
	ctx := context.Background()

	since := time.Now().AddDate(0, 0, -windowDays)

	rows, err := pool.Query(ctx,
		`SELECT id, repository, since_timestamp, until_timestamp, extractor_version,
		        engineering, theoretical, summary, concepts, theoretical_territory,
		        annotation, embedding, git_input, raw_output, created_at
		 FROM journal_entries
		 WHERE embedding IS NOT NULL
		   AND created_at >= $1
		 ORDER BY since_timestamp DESC`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent entries with embeddings: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// GetRecentlyActivatedStandingSlugs returns the set of standing document slugs
// that appear in entry_standing_associations for entries within the last days days.
func GetRecentlyActivatedStandingSlugs(pool *pgxpool.Pool, days int) ([]string, error) {
	ctx := context.Background()

	since := time.Now().AddDate(0, 0, -days)

	rows, err := pool.Query(ctx,
		`SELECT DISTINCT esa.standing_slug
		 FROM entry_standing_associations esa
		 JOIN journal_entries je ON je.id = esa.entry_id
		 WHERE je.created_at >= $1`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get recently activated standing slugs: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("failed to scan standing slug: %w", err)
		}
		slugs = append(slugs, slug)
	}
	return slugs, rows.Err()
}

// GetRecentEntriesInStandingSpace returns entries within the lookback window as points
// in standing-document space. Each point carries a map of standing_slug -> similarity.
// Uses created_at for recency (same semantics as GetRecentEntriesWithEmbeddings).
func GetRecentEntriesInStandingSpace(pool *pgxpool.Pool, windowDays int) ([]EntrySpacePoint, error) {
	ctx := context.Background()
	since := time.Now().AddDate(0, 0, -windowDays)

	rows, err := pool.Query(ctx,
		`SELECT je.id, je.repository, je.since_timestamp, je.created_at,
		        esa.standing_slug, esa.similarity
		 FROM journal_entries je
		 JOIN entry_standing_associations esa ON esa.entry_id = je.id
		 WHERE je.created_at >= $1
		 ORDER BY je.id, esa.standing_slug`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries in standing space: %w", err)
	}
	defer rows.Close()

	pointMap := make(map[int64]*EntrySpacePoint)
	var order []int64

	for rows.Next() {
		var entryID int64
		var repo string
		var sinceTS, createdAt time.Time
		var slug string
		var sim float32

		if err := rows.Scan(&entryID, &repo, &sinceTS, &createdAt, &slug, &sim); err != nil {
			return nil, fmt.Errorf("failed to scan entry space point: %w", err)
		}

		pt, exists := pointMap[entryID]
		if !exists {
			pt = &EntrySpacePoint{
				EntryID:        entryID,
				Repository:     repo,
				SinceTimestamp: sinceTS,
				CreatedAt:      createdAt,
				Coords:         make(map[string]float32),
			}
			pointMap[entryID] = pt
			order = append(order, entryID)
		}
		pt.Coords[slug] = sim
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	points := make([]EntrySpacePoint, 0, len(order))
	for _, id := range order {
		points = append(points, *pointMap[id])
	}
	return points, nil
}

func scanEntries(rows pgx.Rows) ([]JournalEntry, error) {
	var entries []JournalEntry
	for rows.Next() {
		var e JournalEntry
		var gitInput *string
		if err := rows.Scan(&e.ID, &e.Repository, &e.SinceTimestamp, &e.UntilTimestamp,
			&e.ExtractorVersion,
			&e.Engineering, &e.Theoretical, &e.Summary, &e.Concepts, &e.TheoreticalTerritory,
			&e.Annotation, &e.Embedding, &gitInput, &e.RawOutput, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan journal entry: %w", err)
		}
		if gitInput != nil {
			e.GitInput = *gitInput
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
