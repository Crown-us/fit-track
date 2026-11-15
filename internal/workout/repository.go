package workout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Crown-us/fit-track/internal/domain"
)

const selectColumns = `id, activity, duration_min, distance_km, performed_on, calories_burned`

// dateFilter treats an empty bound as unbounded so one query serves every
// combination of from/to.
const dateFilter = `user_id = ? AND (? = '' OR performed_on >= ?) AND (? = '' OR performed_on <= ?)`

// Workout is one logged exercise session.
type Workout struct {
	ID             int64    `json:"id"`
	Activity       Activity `json:"activity"`
	DurationMin    float64  `json:"duration_min"`
	DistanceKm     *float64 `json:"distance_km"`
	Date           string   `json:"date"`
	CaloriesBurned float64  `json:"calories_burned"`
}

// DayTotals aggregates the workouts of one calendar day.
type DayTotals struct {
	CaloriesBurned float64
	WorkoutCount   int
}

// Repository persists workouts.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorkout(row rowScanner) (Workout, error) {
	var w Workout
	err := row.Scan(&w.ID, &w.Activity, &w.DurationMin, &w.DistanceKm, &w.Date, &w.CaloriesBurned)
	return w, err
}

func dateArgs(userID int64, dates domain.DateRange) []any {
	return []any{userID, dates.From, dates.From, dates.To, dates.To}
}

// List returns one page of userID's workouts, newest first, and the total.
func (r *Repository) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Workout, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workouts WHERE `+dateFilter, dateArgs(userID, dates)...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count workouts: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectColumns+` FROM workouts WHERE `+dateFilter+`
		ORDER BY performed_on DESC, id DESC LIMIT ? OFFSET ?`,
		append(dateArgs(userID, dates), page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("select workouts: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	workouts := make([]Workout, 0, page.Limit)
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan workout: %w", err)
		}
		workouts = append(workouts, w)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate workouts: %w", err)
	}
	return workouts, total, nil
}

// Get returns the workout with id if it belongs to userID.
func (r *Repository) Get(ctx context.Context, userID, id int64) (Workout, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+selectColumns+` FROM workouts WHERE id = ? AND user_id = ?`, id, userID)
	w, err := scanWorkout(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Workout{}, domain.ErrNotFound
	}
	if err != nil {
		return Workout{}, fmt.Errorf("select workout: %w", err)
	}
	return w, nil
}

// Create inserts a workout for userID and returns its ID.
func (r *Repository) Create(ctx context.Context, userID int64, w Workout) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO workouts (user_id, activity, duration_min, distance_km, performed_on, calories_burned)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, w.Activity, w.DurationMin, w.DistanceKm, w.Date, w.CaloriesBurned)
	if err != nil {
		return 0, fmt.Errorf("insert workout: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read workout id: %w", err)
	}
	return id, nil
}

// Update replaces the workout w.ID of userID; it returns domain.ErrNotFound
// when no such workout belongs to the user.
func (r *Repository) Update(ctx context.Context, userID int64, w Workout) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE workouts SET activity = ?, duration_min = ?, distance_km = ?,
			performed_on = ?, calories_burned = ?
		WHERE id = ? AND user_id = ?`,
		w.Activity, w.DurationMin, w.DistanceKm, w.Date, w.CaloriesBurned, w.ID, userID)
	if err != nil {
		return fmt.Errorf("update workout: %w", err)
	}
	return requireAffected(result)
}

// Delete removes the workout with id if it belongs to userID.
func (r *Repository) Delete(ctx context.Context, userID, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM workouts WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	return requireAffected(result)
}

// DailyTotals returns calories burned and session counts per day in dates.
func (r *Repository) DailyTotals(ctx context.Context, userID int64, dates domain.DateRange) (map[string]DayTotals, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT performed_on, SUM(calories_burned), COUNT(*)
		FROM workouts WHERE `+dateFilter+` GROUP BY performed_on`, dateArgs(userID, dates)...)
	if err != nil {
		return nil, fmt.Errorf("select workout totals: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	totals := make(map[string]DayTotals)
	for rows.Next() {
		var date string
		var day DayTotals
		if err := rows.Scan(&date, &day.CaloriesBurned, &day.WorkoutCount); err != nil {
			return nil, fmt.Errorf("scan workout totals: %w", err)
		}
		totals[date] = day
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workout totals: %w", err)
	}
	return totals, nil
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
