package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// SQLiteUsersRepository handles user-related DB queries.
type SQLiteUsersRepository struct {
	db *sqlx.DB
}

// NewSQLiteUsersRepository creates a new SQLiteUsersRepository.
func NewSQLiteUsersRepository(db *sqlx.DB) *SQLiteUsersRepository {
	return &SQLiteUsersRepository{db: db}
}

// SearchUsers returns users whose name or email contains q (case-insensitive),
// excluding the viewer themselves and soft-deleted accounts.
func (r *SQLiteUsersRepository) SearchUsers(ctx context.Context, viewerID, q string, limit, offset int) ([]model.UserSummary, int, error) {
	pattern := "%" + strings.ToLower(q) + "%"

	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*)
		 FROM users
		 WHERE deleted_at IS NULL
		   AND id != ?
		   AND (LOWER(name) LIKE ? OR LOWER(email) LIKE ?)`,
		viewerID, pattern, pattern,
	); err != nil {
		return nil, 0, fmt.Errorf("count user search: %w", err)
	}

	var rows []sqliteUserSummaryRow
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT id, name, COALESCE(avatar_url, '') AS avatar_url
		 FROM users
		 WHERE deleted_at IS NULL
		   AND id != ?
		   AND (LOWER(name) LIKE ? OR LOWER(email) LIKE ?)
		 ORDER BY name ASC
		 LIMIT ? OFFSET ?`,
		viewerID, pattern, pattern, limit, offset,
	); err != nil {
		return nil, 0, fmt.Errorf("search users: %w", err)
	}

	users := make([]model.UserSummary, 0, len(rows))
	for i := range rows {
		users = append(users, rows[i].toModel())
	}
	return users, total, nil
}
