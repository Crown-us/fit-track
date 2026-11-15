// Package weight keeps each user's body-weight history, so past workouts and
// summaries are computed with the weight the user had at the time.
package weight

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Crown-us/fit-track/internal/domain"
)

// dateFilter treats an empty bound as unbounded so one query serves every
// combination of from/to.
const dateFilter = `user_id = ? AND (? = '' OR logged_on >= ?) AND (? = '' OR logged_on <= ?)`

// syncProfileWeight keeps profiles.weight_kg equal to the most recent log.
// The profile column is the "current weight" shown to the user; the history
// is the source of truth, so the two are updated in the same transaction.
const syncProfileWeight = `
	UPDATE profiles SET weight_kg = (
		SELECT weight_kg FROM weight_logs WHERE user_id = ? ORDER BY logged_on DESC LIMIT 1
	) WHERE user_id = ? AND EXISTS (SELECT 1 FROM weight_logs WHERE user_id = ?)`

// ErrLastEntry is returned when a delete would leave a user without any
// weight, which workouts and summaries need to compute with.
var ErrLastEntry = errors.New("the only weight entry cannot be deleted")

// Entry is the weight recorded for one calendar day.
type Entry struct {
	Date     string  `json:"date"`
	WeightKg float64 `json:"weight_kg"`
}

// Repository persists weight logs.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns one page of userID's entries, newest first, and the total.
func (r *Repository) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Entry, int, error) {
	args := []any{userID, dates.From, dates.From, dates.To, dates.To}
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM weight_logs WHERE `+dateFilter, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count weight logs: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT logged_on, weight_kg FROM weight_logs WHERE `+dateFilter+`
		ORDER BY logged_on DESC LIMIT ? OFFSET ?`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("select weight logs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	entries := make([]Entry, 0, page.Limit)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.Date, &entry.WeightKg); err != nil {
			return nil, 0, fmt.Errorf("scan weight log: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate weight logs: %w", err)
	}
	return entries, total, nil
}

// Upsert records entry for userID, replacing an entry of the same day.
func (r *Repository) Upsert(ctx context.Context, userID int64, entry Entry) error {
	return r.inTx(ctx, userID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO weight_logs (user_id, logged_on, weight_kg) VALUES (?, ?, ?)
			ON CONFLICT (user_id, logged_on) DO UPDATE SET weight_kg = excluded.weight_kg`,
			userID, entry.Date, entry.WeightKg)
		if err != nil {
			return fmt.Errorf("upsert weight log: %w", err)
		}
		return nil
	})
}

// Delete removes userID's entry of date. It returns domain.ErrNotFound when
// there is none and ErrLastEntry when it is the user's only entry.
func (r *Repository) Delete(ctx context.Context, userID int64, date string) error {
	return r.inTx(ctx, userID, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM weight_logs WHERE user_id = ? AND logged_on = ?`, userID, date)
		if err != nil {
			return fmt.Errorf("delete weight log: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read affected rows: %w", err)
		}
		if affected == 0 {
			return domain.ErrNotFound
		}
		var remaining int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM weight_logs WHERE user_id = ?`, userID).Scan(&remaining)
		if err != nil {
			return fmt.Errorf("count weight logs: %w", err)
		}
		if remaining == 0 {
			return ErrLastEntry
		}
		return nil
	})
}

// On returns the weight that applies on date: the most recent entry on or
// before it, or the earliest entry when date precedes the whole history
// (the best available estimate). It returns domain.ErrNotFound when userID
// has no entries at all.
func (r *Repository) On(ctx context.Context, userID int64, date string) (float64, error) {
	var weightKg float64
	err := r.db.QueryRowContext(ctx, `
		SELECT weight_kg FROM weight_logs WHERE user_id = ?
		ORDER BY (logged_on <= ?) DESC,
			CASE WHEN logged_on <= ? THEN logged_on END DESC,
			logged_on ASC
		LIMIT 1`, userID, date, date).Scan(&weightKg)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("select weight on date: %w", err)
	}
	return weightKg, nil
}

// inTx runs change and then re-syncs the profile's current weight, atomically.
func (r *Repository) inTx(ctx context.Context, userID int64, change func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := change(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.ExecContext(ctx, syncProfileWeight, userID, userID, userID); err != nil {
		return errors.Join(fmt.Errorf("sync profile weight: %w", err), tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
