package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

type sqliteSocialPrefsRow struct {
	UserID      string `db:"user_id"`
	MutePosts   bool   `db:"mute_posts"`
	MuteFollows bool   `db:"mute_follows"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

// SQLiteSocialPrefsRepository implements SocialNotificationPrefsStore backed by SQLite.
type SQLiteSocialPrefsRepository struct {
	db *sqlx.DB
}

// NewSQLiteSocialPrefsRepository creates a new SQLiteSocialPrefsRepository.
func NewSQLiteSocialPrefsRepository(db *sqlx.DB) *SQLiteSocialPrefsRepository {
	return &SQLiteSocialPrefsRepository{db: db}
}

// GetPrefs returns the social notification prefs for the given user.
// Returns zero-value prefs (all false) when no row exists — not an error.
func (r *SQLiteSocialPrefsRepository) GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error) {
	var row sqliteSocialPrefsRow
	err := r.db.GetContext(ctx, &row,
		`SELECT user_id, mute_posts, mute_follows, created_at, updated_at
		 FROM social_notification_prefs WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SocialNotificationPrefs{UserID: userID}, nil
		}
		return model.SocialNotificationPrefs{}, fmt.Errorf("get social prefs: %w", err)
	}
	return model.SocialNotificationPrefs{
		UserID:      row.UserID,
		MutePosts:   row.MutePosts,
		MuteFollows: row.MuteFollows,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// UpsertPrefs creates or updates the social notification prefs for the given user.
// Uses INSERT ... ON CONFLICT to preserve created_at on updates.
func (r *SQLiteSocialPrefsRepository) UpsertPrefs(ctx context.Context, prefs model.SocialNotificationPrefs) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO social_notification_prefs (user_id, mute_posts, mute_follows, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		     mute_posts   = excluded.mute_posts,
		     mute_follows = excluded.mute_follows,
		     updated_at   = excluded.updated_at`,
		prefs.UserID, prefs.MutePosts, prefs.MuteFollows, prefs.CreatedAt, prefs.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert social prefs: %w", err)
	}
	return nil
}
