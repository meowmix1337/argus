package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformdb "github.com/meowmix1337/argus/backend/internal/platform/database"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// sqlitePostRow mirrors the posts table joined with user info and like status.
type sqlitePostRow struct {
	ID           string  `db:"id"`
	UserID       string  `db:"user_id"`
	UserName     string  `db:"user_name"`
	UserAvatar   string  `db:"user_avatar"`
	Content      string  `db:"content"`
	ParentPostID *string `db:"parent_post_id"`
	LikeCount    int     `db:"like_count"`
	MediaURLs    *string `db:"media_urls"`
	LikedByMe    int     `db:"liked_by_me"`
	CreatedAt    string  `db:"created_at"`
}

func (r *sqlitePostRow) toModel() model.Post {
	avatar := r.UserAvatar
	return model.Post{
		ID:           r.ID,
		UserID:       r.UserID,
		UserName:     r.UserName,
		UserAvatar:   avatar,
		Content:      r.Content,
		ParentPostID: r.ParentPostID,
		LikeCount:    r.LikeCount,
		MediaURLs:    r.MediaURLs,
		LikedByMe:    r.LikedByMe == 1,
		CreatedAt:    r.CreatedAt,
	}
}

// SQLitePostsRepository implements PostStore backed by SQLite.
type SQLitePostsRepository struct {
	db *sqlx.DB
}

// NewSQLitePostsRepository creates a new SQLitePostsRepository.
func NewSQLitePostsRepository(db *sqlx.DB) *SQLitePostsRepository {
	return &SQLitePostsRepository{db: db}
}

func (r *SQLitePostsRepository) Create(ctx context.Context, p model.PostCreate) (model.Post, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO posts (id, user_id, content, parent_post_id, media_urls, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.UserID, p.Content, p.ParentPostID, p.MediaURLs, now, now,
	)
	if err != nil {
		return model.Post{}, fmt.Errorf("insert post: %w", err)
	}
	return r.GetByIDWithLike(ctx, p.ID, p.UserID)
}

func (r *SQLitePostsRepository) GetByID(ctx context.Context, postID string) (model.Post, error) {
	var row sqlitePostRow
	err := r.db.GetContext(ctx, &row,
		`SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
		        p.content, p.parent_post_id, p.like_count, p.media_urls,
		        0 AS liked_by_me, p.created_at
		 FROM posts p
		 JOIN users u ON u.id = p.user_id
		 WHERE p.id = ? AND p.deleted_at IS NULL`,
		postID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Post{}, apperrors.ErrPostNotFound
		}
		return model.Post{}, fmt.Errorf("get post: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLitePostsRepository) GetByIDWithLike(ctx context.Context, postID, viewerID string) (model.Post, error) {
	var row sqlitePostRow
	err := r.db.GetContext(ctx, &row,
		`SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
		        p.content, p.parent_post_id, p.like_count, p.media_urls,
		        CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me,
		        p.created_at
		 FROM posts p
		 JOIN users u ON u.id = p.user_id
		 LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
		 WHERE p.id = ? AND p.deleted_at IS NULL`,
		viewerID, postID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Post{}, apperrors.ErrPostNotFound
		}
		return model.Post{}, fmt.Errorf("get post with like: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLitePostsRepository) Delete(ctx context.Context, postID, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	result, err := r.db.ExecContext(ctx,
		`UPDATE posts SET deleted_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, postID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("soft-delete post: %w", err)
	}
	return result.RowsAffected()
}

func (r *SQLitePostsRepository) Like(ctx context.Context, id, postID, userID string) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)

	// Try to re-activate a soft-deleted like first.
	result, err := r.db.ExecContext(ctx,
		`UPDATE post_likes SET deleted_at = NULL, updated_at = ?
		 WHERE post_id = ? AND user_id = ? AND deleted_at IS NOT NULL`,
		now, postID, userID,
	)
	if err != nil {
		return fmt.Errorf("reactivate like: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	// No soft-deleted row — insert a fresh like.
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO post_likes (id, post_id, user_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, postID, userID, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert like: %w", err)
	}
	return nil
}

func (r *SQLitePostsRepository) Unlike(ctx context.Context, postID, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	result, err := r.db.ExecContext(ctx,
		`UPDATE post_likes SET deleted_at = ?, updated_at = ?
		 WHERE post_id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, postID, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("soft-delete like: %w", err)
	}
	return result.RowsAffected()
}

func (r *SQLitePostsRepository) ListByUser(ctx context.Context, authorID, viewerID string, limit, offset int) ([]model.Post, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM posts
		 WHERE user_id = ? AND deleted_at IS NULL
		   AND (user_id = ? OR EXISTS (
		       SELECT 1 FROM followers
		       WHERE follower_id = ? AND following_id = posts.user_id AND deleted_at IS NULL
		   ))`,
		authorID, viewerID, viewerID,
	); err != nil {
		return nil, 0, fmt.Errorf("count posts by user: %w", err)
	}

	var rows []sqlitePostRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
		        p.content, p.parent_post_id, p.like_count, p.media_urls,
		        CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me,
		        p.created_at
		 FROM posts p
		 JOIN users u ON u.id = p.user_id
		 LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
		 WHERE p.user_id = ? AND p.deleted_at IS NULL
		   AND (p.user_id = ? OR EXISTS (
		       SELECT 1 FROM followers
		       WHERE follower_id = ? AND following_id = p.user_id AND deleted_at IS NULL
		   ))
		 ORDER BY p.created_at DESC
		 LIMIT ? OFFSET ?`,
		viewerID, authorID, viewerID, viewerID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts by user: %w", err)
	}

	posts := make([]model.Post, 0, len(rows))
	for i := range rows {
		posts = append(posts, rows[i].toModel())
	}
	return posts, total, nil
}

func (r *SQLitePostsRepository) Search(ctx context.Context, query, viewerID string, limit, offset int) ([]model.Post, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*)
		 FROM posts_fts fts
		 JOIN posts p ON p.id = fts.post_id
		 WHERE posts_fts MATCH ?
		   AND p.deleted_at IS NULL
		   AND (p.user_id = ? OR EXISTS (
		       SELECT 1 FROM followers
		       WHERE follower_id = ? AND following_id = p.user_id AND deleted_at IS NULL
		   ))`,
		query, viewerID, viewerID,
	); err != nil {
		return nil, 0, fmt.Errorf("count search results: %w", err)
	}

	var rows []sqlitePostRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT p.id, p.user_id, u.name AS user_name, COALESCE(u.avatar_url, '') AS user_avatar,
		        p.content, p.parent_post_id, p.like_count, p.media_urls,
		        CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me,
		        p.created_at
		 FROM posts_fts fts
		 JOIN posts p ON p.id = fts.post_id
		 JOIN users u ON u.id = p.user_id
		 LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
		 WHERE posts_fts MATCH ?
		   AND p.deleted_at IS NULL
		   AND (p.user_id = ? OR EXISTS (
		       SELECT 1 FROM followers
		       WHERE follower_id = ? AND following_id = p.user_id AND deleted_at IS NULL
		   ))
		 ORDER BY fts.rank
		 LIMIT ? OFFSET ?`,
		viewerID, query, viewerID, viewerID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("search posts: %w", err)
	}

	posts := make([]model.Post, 0, len(rows))
	for i := range rows {
		posts = append(posts, rows[i].toModel())
	}
	return posts, total, nil
}

// ListPostIDsByAuthor returns up to limit post IDs and timestamps for the given
// author, newest first. Used by the follow backfill consumer.
func (r *SQLitePostsRepository) ListPostIDsByAuthor(ctx context.Context, authorID string, limit int) ([]model.PostRef, error) {
	type row struct {
		ID        string `db:"id"`
		CreatedAt string `db:"created_at"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT id, created_at
		 FROM posts
		 WHERE user_id = ? AND deleted_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT ?`,
		authorID, limit,
	); err != nil {
		return nil, fmt.Errorf("list post ids by author: %w", err)
	}
	refs := make([]model.PostRef, len(rows))
	for i, row := range rows {
		refs[i] = model.PostRef{ID: row.ID, CreatedAt: row.CreatedAt}
	}
	return refs, nil
}
