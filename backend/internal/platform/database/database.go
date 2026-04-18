package database

import (
	"fmt"
	"net/url"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // register SQLite driver
)

// Open opens (or creates) the SQLite database at path and returns a *sqlx.DB
// configured for a single-writer workload with WAL mode and foreign key enforcement.
// Pragmas are embedded in the DSN so they apply to every new connection.
func Open(path string) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)",
		url.PathEscape(path),
	)

	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite supports one writer at a time; cap to prevent "database is locked" errors.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // no expiry — connections are cheap for SQLite

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	return db, nil
}
