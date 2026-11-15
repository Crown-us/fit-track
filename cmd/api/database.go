package main

import (
	"context"
	"database/sql"

	"fittrack/internal/store"
)

// prepareDatabase brings the schema up to date before the API serves traffic.
func prepareDatabase(ctx context.Context, db *sql.DB) error {
	return store.Migrate(ctx, db)
}
