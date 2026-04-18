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

type sqliteWatchedRepoRow struct {
	ID            string         `db:"id"`
	UserID        string         `db:"user_id"`
	IntegrationID string         `db:"integration_id"`
	Owner         string         `db:"owner"`
	Repo          string         `db:"repo"`
	WebhookID     string         `db:"webhook_id"`
	WebhookSecret string         `db:"webhook_secret"`
	CreatedAt     string         `db:"created_at"`
	UpdatedAt     string         `db:"updated_at"`
	DeletedAt     sql.NullString `db:"deleted_at"`
}

func (r *sqliteWatchedRepoRow) toModel() model.WatchedRepo {
	m := model.WatchedRepo{
		ID:            r.ID,
		UserID:        r.UserID,
		IntegrationID: r.IntegrationID,
		Owner:         r.Owner,
		Repo:          r.Repo,
		WebhookID:     r.WebhookID,
		WebhookSecret: r.WebhookSecret,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if r.DeletedAt.Valid {
		m.DeletedAt = &r.DeletedAt.String
	}
	return m
}

const watchedRepoColumns = `id, user_id, integration_id, owner, repo, webhook_id, webhook_secret, created_at, updated_at, deleted_at`

// SQLiteWatchedRepoRepository implements WatchedRepoRepository backed by SQLite via sqlx.
type SQLiteWatchedRepoRepository struct {
	db *sqlx.DB
}

// NewSQLiteWatchedRepoRepository creates a new SQLiteWatchedRepoRepository.
func NewSQLiteWatchedRepoRepository(db *sqlx.DB) *SQLiteWatchedRepoRepository {
	return &SQLiteWatchedRepoRepository{db: db}
}

func (r *SQLiteWatchedRepoRepository) Create(ctx context.Context, w model.WatchedRepoCreate) (model.WatchedRepo, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_watched_repos
		 (id, user_id, integration_id, owner, repo, webhook_id, webhook_secret, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.UserID, w.IntegrationID, w.Owner, w.Repo, w.WebhookID, w.WebhookSecret, now, now,
	)
	if err != nil {
		return model.WatchedRepo{}, fmt.Errorf("create watched repo: %w", err)
	}
	return r.GetByID(ctx, w.ID, w.UserID)
}

func (r *SQLiteWatchedRepoRepository) GetByID(ctx context.Context, id, userID string) (model.WatchedRepo, error) {
	var row sqliteWatchedRepoRow
	err := r.db.GetContext(ctx, &row,
		`SELECT `+watchedRepoColumns+`
		 FROM user_watched_repos
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.WatchedRepo{}, apperrors.ErrWatchedRepoNotFound
		}
		return model.WatchedRepo{}, fmt.Errorf("get watched repo: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteWatchedRepoRepository) ListByIntegration(ctx context.Context, integrationID, userID string) ([]model.WatchedRepo, error) {
	var rows []sqliteWatchedRepoRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT `+watchedRepoColumns+`
		 FROM user_watched_repos
		 WHERE integration_id = ? AND user_id = ? AND deleted_at IS NULL
		 ORDER BY created_at ASC`,
		integrationID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list watched repos by integration: %w", err)
	}

	result := make([]model.WatchedRepo, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, nil
}

// GetByOwnerRepo returns all watched repos for the given owner/repo across all users.
// Intentionally omits userID scoping — webhook dispatch receives an owner/repo from the
// payload with no user context, and must match every user watching that repo.
func (r *SQLiteWatchedRepoRepository) GetByOwnerRepo(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error) {
	var rows []sqliteWatchedRepoRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT `+watchedRepoColumns+`
		 FROM user_watched_repos
		 WHERE owner = ? AND repo = ? AND deleted_at IS NULL`,
		owner, repo,
	)
	if err != nil {
		return nil, fmt.Errorf("get watched repos by owner/repo: %w", err)
	}

	result := make([]model.WatchedRepo, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, nil
}

func (r *SQLiteWatchedRepoRepository) Delete(ctx context.Context, id, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE user_watched_repos SET deleted_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete watched repo: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete watched repo rows affected: %w", err)
	}
	return n, nil
}
