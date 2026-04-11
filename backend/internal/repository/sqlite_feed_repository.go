package repository

import (
	"context"
	"fmt"

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
