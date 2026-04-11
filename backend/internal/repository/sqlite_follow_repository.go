package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// sqliteUserSummaryRow mirrors the columns needed for a lightweight user profile.
type sqliteUserSummaryRow struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	AvatarURL string `db:"avatar_url"`
}

func (r *sqliteUserSummaryRow) toModel() model.UserSummary {
	return model.UserSummary{
		ID:        r.ID,
		Name:      r.Name,
		AvatarURL: r.AvatarURL,
	}
}

// SQLiteFollowRepository implements service.FollowStore backed by SQLite.
type SQLiteFollowRepository struct {
	db *sqlx.DB
}

// NewSQLiteFollowRepository creates a new SQLiteFollowRepository.
func NewSQLiteFollowRepository(db *sqlx.DB) *SQLiteFollowRepository {
	return &SQLiteFollowRepository{db: db}
}

func (r *SQLiteFollowRepository) Follow(ctx context.Context, id, followerID, followingID string) error {
	now := time.Now().UTC().Format(timeFormat)

	// Try to re-activate a soft-deleted follow first.
	result, err := r.db.ExecContext(ctx,
		`UPDATE followers SET deleted_at = NULL, updated_at = ?
		 WHERE follower_id = ? AND following_id = ? AND deleted_at IS NOT NULL`,
		now, followerID, followingID,
	)
	if err != nil {
		return fmt.Errorf("reactivate follow: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	// No soft-deleted row — insert a fresh follow.
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO followers (id, follower_id, following_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, followerID, followingID, now, now,
	)
	if err != nil {
		// Unique index violation means already following (race condition).
		return apperrors.ErrAlreadyFollowing
	}
	return nil
}

func (r *SQLiteFollowRepository) Unfollow(ctx context.Context, followerID, followingID string) (int64, error) {
	now := time.Now().UTC().Format(timeFormat)
	result, err := r.db.ExecContext(ctx,
		`UPDATE followers SET deleted_at = ?, updated_at = ?
		 WHERE follower_id = ? AND following_id = ? AND deleted_at IS NULL`,
		now, now, followerID, followingID,
	)
	if err != nil {
		return 0, fmt.Errorf("soft-delete follow: %w", err)
	}
	return result.RowsAffected()
}

func (r *SQLiteFollowRepository) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM followers
		 WHERE follower_id = ? AND following_id = ? AND deleted_at IS NULL`,
		followerID, followingID,
	)
	if err != nil {
		return false, fmt.Errorf("check follow: %w", err)
	}
	return count > 0, nil
}

func (r *SQLiteFollowRepository) ListFollowers(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM followers
		 WHERE following_id = ? AND deleted_at IS NULL`,
		userID,
	); err != nil {
		return nil, 0, fmt.Errorf("count followers: %w", err)
	}

	var rows []sqliteUserSummaryRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT u.id, u.name, COALESCE(u.avatar_url, '') AS avatar_url
		 FROM followers f
		 JOIN users u ON u.id = f.follower_id
		 WHERE f.following_id = ? AND f.deleted_at IS NULL AND u.deleted_at IS NULL
		 ORDER BY f.created_at DESC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list followers: %w", err)
	}

	users := make([]model.UserSummary, 0, len(rows))
	for i := range rows {
		users = append(users, rows[i].toModel())
	}
	return users, total, nil
}

func (r *SQLiteFollowRepository) ListFollowing(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM followers
		 WHERE follower_id = ? AND deleted_at IS NULL`,
		userID,
	); err != nil {
		return nil, 0, fmt.Errorf("count following: %w", err)
	}

	var rows []sqliteUserSummaryRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT u.id, u.name, COALESCE(u.avatar_url, '') AS avatar_url
		 FROM followers f
		 JOIN users u ON u.id = f.following_id
		 WHERE f.follower_id = ? AND f.deleted_at IS NULL AND u.deleted_at IS NULL
		 ORDER BY f.created_at DESC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list following: %w", err)
	}

	users := make([]model.UserSummary, 0, len(rows))
	for i := range rows {
		users = append(users, rows[i].toModel())
	}
	return users, total, nil
}
