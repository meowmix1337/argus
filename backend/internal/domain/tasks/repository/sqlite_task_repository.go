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

// sqliteTaskRow mirrors the tasks table with string timestamps for SQLite scanning.
type sqliteTaskRow struct {
	ID         string `db:"id"`
	UserID     string `db:"user_id"`
	Text       string `db:"text"`
	Done       int    `db:"done"`
	PriorityID string `db:"priority_id"`
	CreatedAt  string `db:"created_at"`
	UpdatedAt  string `db:"updated_at"`
}

func (r *sqliteTaskRow) toModel() model.Task {
	return model.Task{
		ID:       r.ID,
		Text:     r.Text,
		Done:     r.Done == 1,
		Priority: r.PriorityID,
	}
}

// SQLiteTaskRepository implements TaskStore backed by SQLite via sqlx.
type SQLiteTaskRepository struct {
	db *sqlx.DB
}

// NewSQLiteTaskRepository creates a new SQLiteTaskRepository.
func NewSQLiteTaskRepository(db *sqlx.DB) *SQLiteTaskRepository {
	return &SQLiteTaskRepository{db: db}
}

func (r *SQLiteTaskRepository) List(ctx context.Context, userID string, limit, offset int) ([]model.Task, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM tasks WHERE deleted_at IS NULL AND user_id = ?`,
		userID,
	); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	var rows []sqliteTaskRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, user_id, text, done, priority_id, created_at, updated_at
		 FROM tasks
		 WHERE deleted_at IS NULL AND user_id = ?
		 ORDER BY
		   CASE priority_id
		     WHEN 'high' THEN 1
		     WHEN 'medium' THEN 2
		     WHEN 'low' THEN 3
		   END,
		   created_at ASC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}

	result := make([]model.Task, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, total, nil
}

func (r *SQLiteTaskRepository) Get(ctx context.Context, id string, userID string) (model.Task, error) {
	var row sqliteTaskRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, user_id, text, done, priority_id, created_at, updated_at
		 FROM tasks
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Task{}, apperrors.ErrTaskNotFound
		}
		return model.Task{}, fmt.Errorf("get task: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteTaskRepository) Create(ctx context.Context, t model.TaskCreate) (model.Task, error) {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	doneInt := 0
	if t.Done {
		doneInt = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (id, user_id, text, done, priority_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.Text, doneInt, t.PriorityID, now, now,
	)
	if err != nil {
		return model.Task{}, fmt.Errorf("create task: %w", err)
	}
	return r.Get(ctx, t.ID, t.UserID)
}

func (r *SQLiteTaskRepository) Update(ctx context.Context, id string, userID string, u model.TaskUpdate) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)

	setClauses := []string{"updated_at = ?"}
	args := []interface{}{now}

	if u.Done != nil {
		doneInt := 0
		if *u.Done {
			doneInt = 1
		}
		setClauses = append(setClauses, "done = ?")
		args = append(args, doneInt)
	}
	if u.Text != nil {
		setClauses = append(setClauses, "text = ?")
		args = append(args, *u.Text)
	}
	if u.PriorityID != nil {
		setClauses = append(setClauses, "priority_id = ?")
		args = append(args, *u.PriorityID)
	}

	query := "UPDATE tasks SET " + strings.Join(setClauses, ", ") +
		" WHERE id = ? AND user_id = ? AND deleted_at IS NULL"
	args = append(args, id, userID)

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (r *SQLiteTaskRepository) Delete(ctx context.Context, id string, userID string) error {
	now := time.Now().UTC().Format(platformdb.TimeFormat)
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET deleted_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}
