package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// sqliteBillRow mirrors the bills table with nullable fields for SQLite scanning.
type sqliteBillRow struct {
	ID             string          `db:"id"`
	UserID         string          `db:"user_id"`
	Name           string          `db:"name"`
	Amount         sql.NullFloat64 `db:"amount"`
	CategoryID     string          `db:"category_id"`
	RecurrenceType string          `db:"recurrence_type"`
	DueDate        sql.NullString  `db:"due_date"`
	DueDay         sql.NullInt64   `db:"due_day"`
	DueMonth       sql.NullInt64   `db:"due_month"`
	AnchorDate     sql.NullString  `db:"anchor_date"`
	Notes          sql.NullString  `db:"notes"`
	CreatedAt      string          `db:"created_at"`
	UpdatedAt      string          `db:"updated_at"`
}

func (r *sqliteBillRow) toModel() model.Bill {
	b := model.Bill{
		ID:             r.ID,
		UserID:         r.UserID,
		Name:           r.Name,
		CategoryID:     r.CategoryID,
		RecurrenceType: r.RecurrenceType,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.Amount.Valid {
		b.Amount = &r.Amount.Float64
	}
	if r.DueDate.Valid {
		b.DueDate = &r.DueDate.String
	}
	if r.DueDay.Valid {
		day := int(r.DueDay.Int64)
		b.DueDay = &day
	}
	if r.DueMonth.Valid {
		month := int(r.DueMonth.Int64)
		b.DueMonth = &month
	}
	if r.AnchorDate.Valid {
		b.AnchorDate = &r.AnchorDate.String
	}
	if r.Notes.Valid {
		b.Notes = &r.Notes.String
	}
	return b
}

const billColumns = `id, user_id, name, amount, category_id, recurrence_type,
	due_date, due_day, due_month, anchor_date, notes, created_at, updated_at`

// SQLiteBillsRepository implements BillRepository backed by SQLite via sqlx.
type SQLiteBillsRepository struct {
	db *sqlx.DB
}

// NewSQLiteBillsRepository creates a new SQLiteBillsRepository.
func NewSQLiteBillsRepository(db *sqlx.DB) *SQLiteBillsRepository {
	return &SQLiteBillsRepository{db: db}
}

func (r *SQLiteBillsRepository) List(ctx context.Context, userID string, limit, offset int) ([]model.Bill, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM bills WHERE deleted_at IS NULL AND user_id = ?`,
		userID,
	); err != nil {
		return nil, 0, fmt.Errorf("count bills: %w", err)
	}

	var rows []sqliteBillRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT `+billColumns+`
		 FROM bills
		 WHERE deleted_at IS NULL AND user_id = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list bills: %w", err)
	}

	result := make([]model.Bill, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, total, nil
}

func (r *SQLiteBillsRepository) Get(ctx context.Context, id string, userID string) (model.Bill, error) {
	var row sqliteBillRow
	err := r.db.GetContext(ctx, &row,
		`SELECT `+billColumns+`
		 FROM bills
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		id, userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Bill{}, fmt.Errorf("get bill: %w", sql.ErrNoRows)
		}
		return model.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteBillsRepository) Create(ctx context.Context, b model.BillCreate) (model.Bill, error) {
	now := time.Now().UTC().Format(timeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO bills
		 (id, user_id, name, amount, category_id, recurrence_type,
		  due_date, due_day, due_month, anchor_date, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.UserID, b.Name, b.Amount, b.CategoryID, b.RecurrenceType,
		b.DueDate, b.DueDay, b.DueMonth, b.AnchorDate, b.Notes, now, now,
	)
	if err != nil {
		return model.Bill{}, fmt.Errorf("create bill: %w", err)
	}
	return r.Get(ctx, b.ID, b.UserID)
}

func (r *SQLiteBillsRepository) Update(ctx context.Context, id string, userID string, u model.BillUpdate) (int64, error) {
	now := time.Now().UTC().Format(timeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE bills
		 SET name = ?, amount = ?, category_id = ?, recurrence_type = ?,
		     due_date = ?, due_day = ?, due_month = ?, anchor_date = ?, notes = ?,
		     updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		u.Name, u.Amount, u.CategoryID, u.RecurrenceType,
		u.DueDate, u.DueDay, u.DueMonth, u.AnchorDate, u.Notes,
		now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("update bill: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("update bill rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteBillsRepository) Delete(ctx context.Context, id string, userID string) (int64, error) {
	now := time.Now().UTC().Format(timeFormat)
	res, err := r.db.ExecContext(ctx,
		`UPDATE bills SET deleted_at = ?, updated_at = ?
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		now, now, id, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete bill: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete bill rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteBillsRepository) ListActive(ctx context.Context, userID string) ([]model.Bill, error) {
	var rows []sqliteBillRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT `+billColumns+`
		 FROM bills
		 WHERE deleted_at IS NULL AND user_id = ?
		 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list active bills: %w", err)
	}

	result := make([]model.Bill, 0, len(rows))
	for i := range rows {
		result = append(result, rows[i].toModel())
	}
	return result, nil
}
