package main

import (
	"context"
	"database/sql"

	"fittrack/internal/food"
	"fittrack/internal/store"
)

// prepareDatabase brings the schema and seed data up to date before the API
// serves traffic.
func prepareDatabase(ctx context.Context, db *sql.DB) error {
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	return food.Seed(ctx, db)
}
