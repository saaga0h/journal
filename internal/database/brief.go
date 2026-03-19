package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BriefHistoryRecord represents a row in brief_history.
type BriefHistoryRecord struct {
	ID             int64
	SessionID      string
	TriggeredAt    time.Time
	TrendSnapshot  json.RawMessage
	ArticleURL     string
	ArticleTitle   string
	RelevanceScore *float32
	SilenceReason  string
	CreatedAt      time.Time
}

// InsertBriefHistory records a brief trigger event and returns the inserted ID.
func InsertBriefHistory(pool *pgxpool.Pool, record *BriefHistoryRecord) (int64, error) {
	ctx := context.Background()

	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO brief_history
		 (session_id, triggered_at, trend_snapshot, article_url, article_title,
		  relevance_score, silence_reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		record.SessionID, record.TriggeredAt, record.TrendSnapshot,
		record.ArticleURL, record.ArticleTitle,
		record.RelevanceScore, record.SilenceReason,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert brief history: %w", err)
	}
	return id, nil
}

// InsertBriefFeedback records a read/skip signal for a brief session.
func InsertBriefFeedback(pool *pgxpool.Pool, sessionID, action string) error {
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO brief_feedback (session_id, action, recorded_at)
		 VALUES ($1, $2, NOW())`,
		sessionID, action,
	)
	if err != nil {
		return fmt.Errorf("failed to insert brief feedback: %w", err)
	}
	return nil
}
