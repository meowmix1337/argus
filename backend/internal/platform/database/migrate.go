package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"

	"github.com/meowmix1337/argus/backend/migrations"
)

// Migrate runs all pending database migrations using goose.
// Migration SQL files live in backend/migrations/ and are embedded at build time.
// It accepts a context so callers can enforce a startup timeout.
func Migrate(ctx context.Context, db *sqlx.DB) error {
	// goose.NewProvider expects the FS rooted at the migrations directory.
	// Pass the underlying *sql.DB — goose does not need sqlx.
	provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	_, err = provider.Up(ctx)
	return err
}
