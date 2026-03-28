package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

type sqliteIntegrationRow struct {
	ID               string         `db:"id"`
	UserID           string         `db:"user_id"`
	ProviderID       string         `db:"provider_id"`
	AccessToken      string         `db:"access_token"`
	RefreshToken     sql.NullString `db:"refresh_token"`
	ProviderUserID   string         `db:"provider_user_id"`
	ProviderUsername string         `db:"provider_username"`
	CreatedAt        string         `db:"created_at"`
	UpdatedAt        string         `db:"updated_at"`
	DeletedAt        sql.NullString `db:"deleted_at"`
}

func (r *sqliteIntegrationRow) toModel() model.UserIntegration {
	m := model.UserIntegration{
		ID:               r.ID,
		UserID:           r.UserID,
		ProviderID:       r.ProviderID,
		AccessToken:      r.AccessToken,
		ProviderUserID:   r.ProviderUserID,
		ProviderUsername: r.ProviderUsername,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	if r.RefreshToken.Valid {
		m.RefreshToken = &r.RefreshToken.String
	}
	if r.DeletedAt.Valid {
		m.DeletedAt = &r.DeletedAt.String
	}
	return m
}

const integrationColumns = `id, user_id, provider_id, access_token, refresh_token,
	provider_user_id, provider_username, created_at, updated_at, deleted_at`

// SQLiteIntegrationRepository implements IntegrationRepository backed by SQLite via sqlx.
type SQLiteIntegrationRepository struct {
	db *sqlx.DB
}

// NewSQLiteIntegrationRepository creates a new SQLiteIntegrationRepository.
func NewSQLiteIntegrationRepository(db *sqlx.DB) *SQLiteIntegrationRepository {
	return &SQLiteIntegrationRepository{db: db}
}

func (r *SQLiteIntegrationRepository) Create(ctx context.Context, i model.IntegrationCreate) (model.UserIntegration, error) {
	now := time.Now().UTC().Format(timeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_integrations
		 (id, user_id, provider_id, access_token, refresh_token, provider_user_id, provider_username, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.UserID, i.ProviderID, i.AccessToken, i.RefreshToken, i.ProviderUserID, i.ProviderUsername, now, now,
	)
	if err != nil {
		return model.UserIntegration{}, fmt.Errorf("create integration: %w", err)
	}
	return r.GetByID(ctx, i.ID, i.UserID)
}

func (r *SQLiteIntegrationRepository) GetByUserAndProvider(ctx context.Context, userID, providerID string) (model.UserIntegration, error) {
	var row sqliteIntegrationRow
	err := r.db.GetContext(ctx, &row,
		`SELECT `+integrationColumns+`
		 FROM user_integrations
		 WHERE user_id = ? AND provider_id = ? AND deleted_at IS NULL`,
		userID, providerID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.UserIntegration{}, apperrors.ErrIntegrationNotFound
		}
		return model.UserIntegration{}, fmt.Errorf("get integration by provider: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteIntegrationRepository) GetByID(ctx context.Context, id, userID string) (model.UserIntegration, error) {
	var row sqliteIntegrationRow
	err := r.db.GetContext(ctx, &row,
		`SELECT `+integrationColumns+`
		 FROM user_integrations
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.UserIntegration{}, apperrors.ErrIntegrationNotFound
		}
		return model.UserIntegration{}, fmt.Errorf("get integration: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteIntegrationRepository) Delete(ctx context.Context, id, userID string) (int64, error) {
	now := time.Now().UTC().Format(timeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE user_integrations SET deleted_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete integration: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete integration rows affected: %w", err)
	}
	return n, nil
}
