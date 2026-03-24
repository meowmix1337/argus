package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SQLiteStocksWatchlistRepository implements StocksWatchlistRepository backed by SQLite.
type SQLiteStocksWatchlistRepository struct {
	db *sqlx.DB
}

// NewSQLiteStocksWatchlistRepository creates a new SQLiteStocksWatchlistRepository.
func NewSQLiteStocksWatchlistRepository(db *sqlx.DB) *SQLiteStocksWatchlistRepository {
	return &SQLiteStocksWatchlistRepository{db: db}
}

func (r *SQLiteStocksWatchlistRepository) ListSymbols(ctx context.Context, userID string, limit, offset int) ([]string, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM stocks_watchlist WHERE deleted_at IS NULL AND user_id = ?`,
		userID,
	); err != nil {
		return nil, 0, fmt.Errorf("count watchlist symbols: %w", err)
	}

	query := `SELECT symbol FROM stocks_watchlist WHERE deleted_at IS NULL AND user_id = ? ORDER BY created_at ASC`
	args := []any{userID}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var symbols []string
	if err := r.db.SelectContext(ctx, &symbols, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list watchlist symbols: %w", err)
	}
	return symbols, total, nil
}

func (r *SQLiteStocksWatchlistRepository) Exists(ctx context.Context, userID string, symbol string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(1)
		 FROM stocks_watchlist
		 WHERE user_id = ? AND symbol = ? AND deleted_at IS NULL`,
		userID, symbol,
	)
	if err != nil {
		return false, fmt.Errorf("check watchlist symbol: %w", err)
	}
	return count > 0, nil
}

func (r *SQLiteStocksWatchlistRepository) Add(ctx context.Context, userID string, symbol string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate uuid: %w", err)
	}
	now := time.Now().UTC().Format(timeFormat)

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO stocks_watchlist (id, user_id, symbol, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, symbol) DO UPDATE
		   SET deleted_at = NULL,
		       updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		id.String(), userID, symbol, now, now,
	)
	if err != nil {
		return fmt.Errorf("add to watchlist: %w", err)
	}
	return nil
}

func (r *SQLiteStocksWatchlistRepository) Remove(ctx context.Context, userID string, symbol string) error {
	now := time.Now().UTC().Format(timeFormat)
	_, err := r.db.ExecContext(ctx,
		`UPDATE stocks_watchlist
		 SET deleted_at = ?, updated_at = ?
		 WHERE user_id = ? AND symbol = ? AND deleted_at IS NULL`,
		now, now, userID, symbol,
	)
	if err != nil {
		return fmt.Errorf("remove from watchlist: %w", err)
	}
	return nil
}
