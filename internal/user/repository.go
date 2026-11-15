package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Crown-us/fit-track/internal/domain"
	"github.com/Crown-us/fit-track/internal/store"
)

// Repository persists users and profiles.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new user; it returns domain.ErrConflict when the email is
// already registered.
func (r *Repository) Create(ctx context.Context, email, passwordHash string) (User, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, passwordHash)
	if store.IsUniqueViolation(err) {
		return User{}, domain.ErrConflict
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("read user id: %w", err)
	}
	return User{ID: id, Email: email, PasswordHash: passwordHash}, nil
}

// FindByEmail returns the user registered with email.
func (r *Repository) FindByEmail(ctx context.Context, email string) (User, error) {
	var found User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = ?`, email).
		Scan(&found.ID, &found.Email, &found.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, domain.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user: %w", err)
	}
	return found, nil
}

// Profile returns the profile of userID.
func (r *Repository) Profile(ctx context.Context, userID int64) (Profile, error) {
	profile := Profile{UserID: userID}
	err := r.db.QueryRowContext(ctx, `
		SELECT sex, birth_date, height_cm, weight_kg, activity_level, goal, weekly_workout_goal
		FROM profiles WHERE user_id = ?`, userID).
		Scan(&profile.Sex, &profile.BirthDate, &profile.HeightCm, &profile.WeightKg,
			&profile.ActivityLevel, &profile.Goal, &profile.WeeklyWorkoutGoal)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, domain.ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("select profile: %w", err)
	}
	return profile, nil
}

// SaveProfile creates or replaces the profile of profile.UserID.
func (r *Repository) SaveProfile(ctx context.Context, profile Profile) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profiles
			(user_id, sex, birth_date, height_cm, weight_kg, activity_level, goal, weekly_workout_goal)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			sex = excluded.sex,
			birth_date = excluded.birth_date,
			height_cm = excluded.height_cm,
			weight_kg = excluded.weight_kg,
			activity_level = excluded.activity_level,
			goal = excluded.goal,
			weekly_workout_goal = excluded.weekly_workout_goal,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		profile.UserID, profile.Sex, profile.BirthDate, profile.HeightCm, profile.WeightKg,
		profile.ActivityLevel, profile.Goal, profile.WeeklyWorkoutGoal)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}
