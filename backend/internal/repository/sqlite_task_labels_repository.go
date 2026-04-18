package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformdb "github.com/meowmix1337/argus/backend/internal/platform/database"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// sqliteTaskLabelRow mirrors the task_labels table with string timestamps for SQLite scanning.
type sqliteTaskLabelRow struct {
	ID        string `db:"id"`
	UserID    string `db:"user_id"`
	Name      string `db:"name"`
	Color     string `db:"color"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

func (r *sqliteTaskLabelRow) toModel() (model.TaskLabel, error) {
	createdAt, err := time.Parse(platformdb.TimeFormat, r.CreatedAt)
	if err != nil {
		return model.TaskLabel{}, fmt.Errorf("parse created_at: %w", err)
	}
	return model.TaskLabel{
		ID:        r.ID,
		Name:      r.Name,
		Color:     r.Color,
		CreatedAt: createdAt,
	}, nil
}

// SQLiteTaskLabelsRepository implements TaskLabelRepository backed by SQLite via sqlx.
type SQLiteTaskLabelsRepository struct {
	db *sqlx.DB
}

// NewSQLiteTaskLabelsRepository creates a new SQLiteTaskLabelsRepository.
func NewSQLiteTaskLabelsRepository(db *sqlx.DB) *SQLiteTaskLabelsRepository {
	return &SQLiteTaskLabelsRepository{db: db}
}

func (r *SQLiteTaskLabelsRepository) List(ctx context.Context, userID string) ([]model.TaskLabel, error) {
	var rows []sqliteTaskLabelRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, user_id, name, color, created_at, updated_at
		 FROM task_labels
		 WHERE deleted_at IS NULL AND user_id = ?
		 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list task labels: %w", err)
	}

	result := make([]model.TaskLabel, 0, len(rows))
	for i := range rows {
		lr, err := rows[i].toModel()
		if err != nil {
			return nil, err
		}
		result = append(result, lr)
	}
	return result, nil
}

func (r *SQLiteTaskLabelsRepository) Get(ctx context.Context, id string, userID string) (model.TaskLabel, error) {
	var row sqliteTaskLabelRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, user_id, name, color, created_at, updated_at
		 FROM task_labels
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.TaskLabel{}, apperrors.ErrLabelNotFound
		}
		return model.TaskLabel{}, fmt.Errorf("get task label: %w", err)
	}
	return row.toModel()
}

func (r *SQLiteTaskLabelsRepository) Create(ctx context.Context, l model.TaskLabelCreate) (model.TaskLabel, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO task_labels (id, user_id, name, color, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		l.ID, l.UserID, l.Name, l.Color, now, now,
	)
	if err != nil {
		return model.TaskLabel{}, fmt.Errorf("create task label: %w", err)
	}
	return r.Get(ctx, l.ID, l.UserID)
}

func (r *SQLiteTaskLabelsRepository) Update(ctx context.Context, id string, userID string, u model.TaskLabelUpdate) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)

	setClauses := []string{"updated_at = ?"}
	args := []interface{}{now}

	if u.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *u.Name)
	}
	if u.Color != nil {
		setClauses = append(setClauses, "color = ?")
		args = append(args, *u.Color)
	}

	query := "UPDATE task_labels SET " + strings.Join(setClauses, ", ") +
		" WHERE id = ? AND user_id = ? AND deleted_at IS NULL"
	args = append(args, id, userID)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update task label: %w", err)
	}
	return nil
}

func (r *SQLiteTaskLabelsRepository) Delete(ctx context.Context, id string, userID string) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`UPDATE task_labels SET deleted_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete task label: %w", err)
	}
	return nil
}

func (r *SQLiteTaskLabelsRepository) ListForTask(ctx context.Context, taskID string, userID string) ([]model.TaskLabel, error) {
	var rows []sqliteTaskLabelRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT tl.id, tl.user_id, tl.name, tl.color, tl.created_at, tl.updated_at
		 FROM task_labels tl
		 INNER JOIN task_label_assignments tla ON tla.label_id = tl.id
		 WHERE tla.task_id = ? AND tla.deleted_at IS NULL AND tl.deleted_at IS NULL AND tl.user_id = ?
		 ORDER BY tla.created_at ASC`,
		taskID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list labels for task: %w", err)
	}

	result := make([]model.TaskLabel, 0, len(rows))
	for i := range rows {
		lr, err := rows[i].toModel()
		if err != nil {
			return nil, err
		}
		result = append(result, lr)
	}
	return result, nil
}

func (r *SQLiteTaskLabelsRepository) AssignLabel(ctx context.Context, a model.TaskLabelAssignmentCreate) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO task_label_assignments (id, task_id, label_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		a.ID, a.TaskID, a.LabelID, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return apperrors.ErrLabelAlreadyAssigned
		}
		return fmt.Errorf("assign label: %w", err)
	}
	return nil
}

func (r *SQLiteTaskLabelsRepository) RemoveLabel(ctx context.Context, taskID string, labelID string, userID string) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`UPDATE task_label_assignments
		 SET deleted_at = ?, updated_at = ?
		 WHERE task_id = ? AND label_id = ? AND deleted_at IS NULL
		   AND label_id IN (SELECT id FROM task_labels WHERE user_id = ? AND deleted_at IS NULL)`,
		now, now, taskID, labelID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove label: %w", err)
	}
	return nil
}
