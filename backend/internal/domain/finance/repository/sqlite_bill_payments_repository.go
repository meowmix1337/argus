package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// sqliteBillPaymentRow mirrors bill_payments with nullable fields for SQLite scanning.
type sqliteBillPaymentRow struct {
	ID              string         `db:"id"`
	BillID          string         `db:"bill_id"`
	UserID          string         `db:"user_id"`
	ComputedDueDate string         `db:"computed_due_date"`
	PaidDate        string         `db:"paid_date"`
	Note            sql.NullString `db:"note"`
	CreatedAt       string         `db:"created_at"`
}

func (r *sqliteBillPaymentRow) toModel() model.BillPayment {
	p := model.BillPayment{
		ID:              r.ID,
		BillID:          r.BillID,
		UserID:          r.UserID,
		ComputedDueDate: r.ComputedDueDate,
		PaidDate:        r.PaidDate,
		CreatedAt:       r.CreatedAt,
	}
	if r.Note.Valid {
		p.Note = &r.Note.String
	}
	return p
}

// SQLiteBillPaymentsRepository implements BillPaymentStore using SQLite.
type SQLiteBillPaymentsRepository struct {
	db *sqlx.DB
}

// NewSQLiteBillPaymentsRepository creates a new SQLiteBillPaymentsRepository.
func NewSQLiteBillPaymentsRepository(db *sqlx.DB) *SQLiteBillPaymentsRepository {
	return &SQLiteBillPaymentsRepository{db: db}
}

func (r *SQLiteBillPaymentsRepository) Create(ctx context.Context, p model.BillPaymentCreate) (model.BillPayment, error) {
	const q = `
		INSERT INTO bill_payments (id, bill_id, user_id, computed_due_date, paid_date, note)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	var note sql.NullString
	if p.Note != nil {
		note = sql.NullString{String: *p.Note, Valid: true}
	}
	if _, err := r.db.ExecContext(ctx, q, p.ID, p.BillID, p.UserID, p.ComputedDueDate, p.PaidDate, note); err != nil {
		return model.BillPayment{}, fmt.Errorf("insert bill payment: %w", err)
	}

	const sel = `
		SELECT id, bill_id, user_id, computed_due_date, paid_date, note, created_at
		  FROM bill_payments
		 WHERE id = ?
	`
	var row sqliteBillPaymentRow
	if err := r.db.GetContext(ctx, &row, sel, p.ID); err != nil {
		return model.BillPayment{}, fmt.Errorf("fetch created bill payment: %w", err)
	}
	return row.toModel(), nil
}

func (r *SQLiteBillPaymentsRepository) Delete(ctx context.Context, id string, userID string) (int64, error) {
	const q = `DELETE FROM bill_payments WHERE id = ? AND user_id = ?`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return 0, fmt.Errorf("delete bill payment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

func (r *SQLiteBillPaymentsRepository) ListForYear(ctx context.Context, userID string, year int) ([]model.BillPayment, error) {
	const q = `
		SELECT id, bill_id, user_id, computed_due_date, paid_date, note, created_at
		  FROM bill_payments
		 WHERE user_id = ? AND computed_due_date LIKE ?
		 ORDER BY computed_due_date ASC
	`
	pattern := fmt.Sprintf("%04d-%%", year)
	var rows []sqliteBillPaymentRow
	if err := r.db.SelectContext(ctx, &rows, q, userID, pattern); err != nil {
		return nil, fmt.Errorf("list bill payments for year: %w", err)
	}
	result := make([]model.BillPayment, len(rows))
	for i, row := range rows {
		result[i] = row.toModel()
	}
	return result, nil
}

func (r *SQLiteBillPaymentsRepository) ListForMonth(ctx context.Context, userID string, year, month int) ([]model.BillPayment, error) {
	const q = `
		SELECT id, bill_id, user_id, computed_due_date, paid_date, note, created_at
		  FROM bill_payments
		 WHERE user_id = ? AND computed_due_date LIKE ?
		 ORDER BY computed_due_date ASC
	`
	pattern := fmt.Sprintf("%04d-%02d-%%", year, month)
	var rows []sqliteBillPaymentRow
	if err := r.db.SelectContext(ctx, &rows, q, userID, pattern); err != nil {
		return nil, fmt.Errorf("list bill payments for month: %w", err)
	}
	result := make([]model.BillPayment, len(rows))
	for i, row := range rows {
		result[i] = row.toModel()
	}
	return result, nil
}
