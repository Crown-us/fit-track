// Package store opens the SQLite database, applies migrations and classifies
// driver errors so repositories stay free of driver details.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/Crown-us/fit-track/migrations"
)

// connectionOptions are applied to every pooled connection. Foreign keys are
// off by default in SQLite, WAL lets readers run during a write, and the busy
// timeout plus immediate transactions avoid spurious SQLITE_BUSY errors.
const connectionOptions = "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)&_txlock=immediate"

// Open connects to the SQLite database file at path and verifies it responds.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+connectionOptions)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("ping database: %w", err), db.Close())
	}
	return db, nil
}

// Migrate applies every pending embedded migration.
func Migrate(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// IsUniqueViolation reports whether err is a UNIQUE or PRIMARY KEY violation.
func IsUniqueViolation(err error) bool {
	return hasCode(err, sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY)
}

// IsForeignKeyViolation reports whether err is a FOREIGN KEY violation.
func IsForeignKeyViolation(err error) bool {
	return hasCode(err, sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY)
}

func hasCode(err error, codes ...int) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	for _, code := range codes {
		if sqliteErr.Code() == code {
			return true
		}
	}
	return false
}
