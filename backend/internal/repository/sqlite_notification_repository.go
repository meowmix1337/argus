package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
	platformdb "github.com/meowmix1337/argus/backend/internal/platform/database"
)

type sqliteNotificationRow struct {
	ID               string         `db:"id"`
	UserID           string         `db:"user_id"`
	ProviderID       string         `db:"provider_id"`
	EventTypeID      string         `db:"event_type_id"`
	Title            string         `db:"title"`
	Body             sql.NullString `db:"body"`
	URL              sql.NullString `db:"url"`
	ReferenceID      sql.NullString `db:"reference_id"`
	ReadAt           sql.NullString `db:"read_at"`
	DismissedAt      sql.NullString `db:"dismissed_at"`
	GitHubDeliveryID sql.NullString `db:"github_delivery_id"`
	CreatedAt        string         `db:"created_at"`
	UpdatedAt        string         `db:"updated_at"`
	DeletedAt        sql.NullString `db:"deleted_at"`
}

func (r *sqliteNotificationRow) toModel() model.Notification {
	m := model.Notification{
		ID:          r.ID,
		UserID:      r.UserID,
		ProviderID:  r.ProviderID,
		EventTypeID: r.EventTypeID,
		Title:       r.Title,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if r.Body.Valid {
		m.Body = &r.Body.String
	}
	if r.URL.Valid {
		m.URL = &r.URL.String
	}
	if r.ReferenceID.Valid {
		m.ReferenceID = &r.ReferenceID.String
	}
	if r.ReadAt.Valid {
		m.ReadAt = &r.ReadAt.String
	}
	if r.DismissedAt.Valid {
		m.DismissedAt = &r.DismissedAt.String
	}
	if r.GitHubDeliveryID.Valid {
		m.GitHubDeliveryID = &r.GitHubDeliveryID.String
	}
	if r.DeletedAt.Valid {
		m.DeletedAt = &r.DeletedAt.String
	}
	return m
}

const notificationColumns = `id, user_id, provider_id, event_type_id, title, body, url,
	reference_id, read_at, dismissed_at, github_delivery_id, created_at, updated_at, deleted_at`

// SQLiteNotificationRepository implements NotificationRepository backed by SQLite via sqlx.
type SQLiteNotificationRepository struct {
	db *sqlx.DB
}

// NewSQLiteNotificationRepository creates a new SQLiteNotificationRepository.
func NewSQLiteNotificationRepository(db *sqlx.DB) *SQLiteNotificationRepository {
	return &SQLiteNotificationRepository{db: db}
}

// notificationStateFilter returns the static SQL fragment for the given state filter.
// All returned strings are hard-coded — no user input reaches the query.
func notificationStateFilter(state string) string {
	switch state {
	case "unread":
		return " AND read_at IS NULL AND dismissed_at IS NULL"
	case "read":
		return " AND read_at IS NOT NULL"
	case "dismissed":
		return " AND dismissed_at IS NOT NULL"
	default: // "all"
		return ""
	}
}

func (r *SQLiteNotificationRepository) Create(ctx context.Context, n model.NotificationCreate) (model.Notification, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications
		 (id, user_id, provider_id, event_type_id, title, body, url, reference_id, github_delivery_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.UserID, n.ProviderID, n.EventTypeID, n.Title, n.Body, n.URL, n.ReferenceID, n.GitHubDeliveryID, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return model.Notification{}, apperrors.ErrDuplicateDelivery
		}
		return model.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return r.GetByID(ctx, n.ID, n.UserID)
}

func (r *SQLiteNotificationRepository) List(ctx context.Context, userID, state, query, providerID, eventTypeID string, limit, offset int) ([]model.Notification, int, error) {
	filter := notificationStateFilter(state)

	args := []any{userID}
	if query != "" {
		filter += ` AND (title LIKE ? ESCAPE '!' OR body LIKE ? ESCAPE '!')`
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(query)
		like := "%" + escaped + "%"
		args = append(args, like, like)
	}
	if providerID != "" {
		filter += " AND provider_id = ?"
		args = append(args, providerID)
	}
	if eventTypeID != "" {
		filter += " AND event_type_id = ?"
		args = append(args, eventTypeID)
	}

	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND deleted_at IS NULL`+filter,
		args...,
	); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	selectArgs := append(args, limit, offset)
	var rows []sqliteNotificationRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT `+notificationColumns+`
		 FROM notifications
		 WHERE user_id = ? AND deleted_at IS NULL`+filter+`
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		selectArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}

	result := make([]model.Notification, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, total, nil
}

func (r *SQLiteNotificationRepository) GetByID(ctx context.Context, id, userID string) (model.Notification, error) {
	var row sqliteNotificationRow
	err := r.db.GetContext(ctx, &row,
		`SELECT `+notificationColumns+`
		 FROM notifications
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Notification{}, apperrors.ErrNotificationNotFound
		}
		return model.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteNotificationRepository) MarkRead(ctx context.Context, id, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL AND read_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark notification read: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mark notification read rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteNotificationRepository) MarkDismissed(ctx context.Context, id, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET dismissed_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL AND dismissed_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark notification dismissed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mark notification dismissed rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteNotificationRepository) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read_at = ?, updated_at = ?
		 WHERE user_id = ? AND deleted_at IS NULL AND read_at IS NULL`,
		now, now, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteNotificationRepository) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = ? AND deleted_at IS NULL AND read_at IS NULL AND dismissed_at IS NULL`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}
