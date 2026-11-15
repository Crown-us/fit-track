package main

import (
	"context"
	"database/sql"

	"github.com/Crown-us/fit-track/internal/food"
	"github.com/Crown-us/fit-track/internal/store"
)

// prepareDatabase brings the schema and seed data up to date before the API
// serves traffic.
func prepareDatabase(ctx context.Context, db *sql.DB) error {
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	return food.Seed(ctx, db)
}
