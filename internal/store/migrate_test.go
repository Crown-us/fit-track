package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Crown-us/fit-track/migrations"
)

// initialSchemaVersion is the schema before weight history existed.
const initialSchemaVersion = 1

// TestWeightLogBackfill upgrades a database that already has a profile and
// checks that the profile's weight becomes its first weight-history entry.
func TestWeightLogBackfill(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.UpTo(ctx, initialSchemaVersion); err != nil {
		t.Fatalf("UpTo(%d) error = %v", initialSchemaVersion, err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash) VALUES (1, 'old@example.com', 'hash');
		INSERT INTO profiles (user_id, sex, birth_date, height_cm, weight_kg, activity_level, goal,
			weekly_workout_goal, updated_at)
		VALUES (1, 'female', '1998-03-02', 160, 55.5, 'light', 'maintain', 3, '2026-10-01T08:30:00Z');`)
	if err != nil {
		t.Fatalf("insert pre-upgrade data: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var loggedOn string
	var weightKg float64
	err = db.QueryRowContext(ctx, `SELECT logged_on, weight_kg FROM weight_logs WHERE user_id = 1`).
		Scan(&loggedOn, &weightKg)
	if err != nil {
		t.Fatalf("select backfilled weight: %v", err)
	}
	if loggedOn != "2026-10-01" || weightKg != 55.5 {
		t.Errorf("backfilled entry = %s %v, want 2026-10-01 55.5", loggedOn, weightKg)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "twice.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	for attempt := range 2 {
		if err := Migrate(ctx, db); err != nil {
			t.Fatalf("Migrate() attempt %d error = %v", attempt+1, err)
		}
	}
}
