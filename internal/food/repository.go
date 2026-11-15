package food

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"fittrack/internal/domain"
	"fittrack/internal/store"
)

const selectColumns = `id, owner_id, name, kcal_per_100g, protein_g_per_100g,
	carbs_g_per_100g, fat_g_per_100g, source, verification_status`

// visibleFilter limits rows to global foods plus the caller's own.
const visibleFilter = `(owner_id IS NULL OR owner_id = ?)`

// Repository persists foods.
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

func scanFood(row rowScanner) (Food, error) {
	var f Food
	err := row.Scan(&f.ID, &f.OwnerID, &f.Name, &f.KcalPer100g, &f.ProteinGPer100g,
		&f.CarbsGPer100g, &f.FatGPer100g, &f.Source, &f.VerificationStatus)
	f.IsCustom = f.OwnerID != nil
	return f, err
}

// List returns one page of the foods visible to userID whose name contains
// query, plus the total number of matches.
func (r *Repository) List(ctx context.Context, userID int64, query string, page domain.Page) ([]Food, int, error) {
	pattern := "%" + query + "%"
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM foods WHERE `+visibleFilter+` AND name LIKE ?`, userID, pattern).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count foods: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+selectColumns+` FROM foods WHERE `+visibleFilter+` AND name LIKE ?
		ORDER BY name, id LIMIT ? OFFSET ?`, userID, pattern, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("select foods: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	foods := make([]Food, 0, page.Limit)
	for rows.Next() {
		f, err := scanFood(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan food: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate foods: %w", err)
	}
	return foods, total, nil
}

// Get returns the food with id if it is visible to userID.
func (r *Repository) Get(ctx context.Context, userID, id int64) (Food, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+selectColumns+` FROM foods WHERE id = ? AND `+visibleFilter, id, userID)
	f, err := scanFood(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Food{}, domain.ErrNotFound
	}
	if err != nil {
		return Food{}, fmt.Errorf("select food: %w", err)
	}
	return f, nil
}

// Create inserts a custom food owned by userID.
func (r *Repository) Create(ctx context.Context, userID int64, input Input) (Food, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO foods (owner_id, name, kcal_per_100g, protein_g_per_100g,
			carbs_g_per_100g, fat_g_per_100g, source, verification_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, input.Name, input.KcalPer100g, input.ProteinGPer100g,
		input.CarbsGPer100g, input.FatGPer100g, sourceUser, StatusUserProvided)
	if err != nil {
		return Food{}, fmt.Errorf("insert food: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Food{}, fmt.Errorf("read food id: %w", err)
	}
	return r.Get(ctx, userID, id)
}

// Update replaces the values of a custom food owned by userID.
func (r *Repository) Update(ctx context.Context, userID, id int64, input Input) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE foods SET name = ?, kcal_per_100g = ?, protein_g_per_100g = ?,
			carbs_g_per_100g = ?, fat_g_per_100g = ?
		WHERE id = ? AND owner_id = ?`,
		input.Name, input.KcalPer100g, input.ProteinGPer100g,
		input.CarbsGPer100g, input.FatGPer100g, id, userID)
	if err != nil {
		return fmt.Errorf("update food: %w", err)
	}
	return nil
}

// Delete removes a custom food owned by userID. It returns domain.ErrConflict
// while a logged meal still references the food.
func (r *Repository) Delete(ctx context.Context, userID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM foods WHERE id = ? AND owner_id = ?`, id, userID)
	if store.IsForeignKeyViolation(err) {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("delete food: %w", err)
	}
	return nil
}
