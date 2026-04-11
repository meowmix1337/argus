package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// SQLiteFeedRepository implements service.FeedStore backed by SQLite.
type SQLiteFeedRepository struct {
	db *sqlx.DB
}

// NewSQLiteFeedRepository creates a new SQLiteFeedRepository.
func NewSQLiteFeedRepository(db *sqlx.DB) *SQLiteFeedRepository {
	return &SQLiteFeedRepository{db: db}
}

func (r *SQLiteFeedRepository) ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	// Build the base query: posts from users the viewer follows + viewer's own posts.
	query := `SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
	                 p.content, p.parent_post_id, p.like_count, p.media_urls,
	                 CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me,
	                 p.created_at
	          FROM posts p
	          JOIN users u ON u.id = p.user_id
	          LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
	          WHERE p.deleted_at IS NULL
	            AND (p.user_id = ? OR EXISTS (
	                SELECT 1 FROM followers
	                WHERE follower_id = ? AND following_id = p.user_id AND deleted_at IS NULL
	            ))`

	args := []any{viewerID, viewerID, viewerID}

	// Cursor-based pagination: created_at DESC with id as tie-breaker.
	if cursor != nil {
		query += ` AND (p.created_at < ? OR (p.created_at = ? AND p.id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	query += ` ORDER BY p.created_at DESC, p.id DESC LIMIT ?`
	args = append(args, limit)

	var rows []sqlitePostRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}

	posts := make([]model.Post, 0, len(rows))
	for i := range rows {
		posts = append(posts, rows[i].toModel())
	}
	return posts, nil
}

// CountUserFeedForUser returns the number of rows in user_feed for the given user.
func (r *SQLiteFeedRepository) CountUserFeedForUser(ctx context.Context, userID string) (int, error) {
	var count int
	if err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM user_feed WHERE user_id = ?`, userID,
	); err != nil {
		return 0, fmt.Errorf("count user feed: %w", err)
	}
	return count, nil
}

// ListFeedMaterialized reads the feed from the pre-computed user_feed table.
func (r *SQLiteFeedRepository) ListFeedMaterialized(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	query := `SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
	                 p.content, p.parent_post_id, p.like_count, p.media_urls,
	                 CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me,
	                 uf.created_at
	          FROM user_feed uf
	          JOIN posts p ON p.id = uf.post_id
	          JOIN users u ON u.id = p.user_id
	          LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
	          WHERE uf.user_id = ?
	            AND p.deleted_at IS NULL`

	args := []any{viewerID, viewerID}

	if cursor != nil {
		query += ` AND (uf.created_at < ? OR (uf.created_at = ? AND uf.id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}

	query += ` ORDER BY uf.created_at DESC, uf.id DESC LIMIT ?`
	args = append(args, limit)

	var rows []sqlitePostRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list feed materialized: %w", err)
	}

	posts := make([]model.Post, 0, len(rows))
	for i := range rows {
		posts = append(posts, rows[i].toModel())
	}
	return posts, nil
}

// BulkInsertUserFeed inserts feed rows using INSERT OR IGNORE to skip duplicates.
func (r *SQLiteFeedRepository) BulkInsertUserFeed(ctx context.Context, rows []model.UserFeedRow) error {
	if len(rows) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(`INSERT OR IGNORE INTO user_feed (id, user_id, post_id, created_at) VALUES `)
	args := make([]any, 0, len(rows)*4)
	for i, row := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(?, ?, ?, ?)")
		args = append(args, row.ID, row.UserID, row.PostID, row.CreatedAt)
	}

	if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
		return fmt.Errorf("bulk insert user feed: %w", err)
	}
	return nil
}
