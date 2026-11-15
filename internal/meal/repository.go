package meal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"fittrack/internal/domain"
)

// dateFilter treats an empty bound as unbounded so one query serves every
// combination of from/to.
const dateFilter = `m.user_id = ? AND (? = '' OR m.eaten_on >= ?) AND (? = '' OR m.eaten_on <= ?)`

// Repository persists meals and their items.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// datedItem is one eaten food together with the day it was eaten on.
type datedItem struct {
	Date    string
	MealID  int64
	Item    Item
	Per100g Nutrition
}

func dateArgs(userID int64, dates domain.DateRange) []any {
	return []any{userID, dates.From, dates.From, dates.To, dates.To}
}

// inTx runs fn in a transaction, rolling back when fn fails.
func (r *Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func insertItems(ctx context.Context, tx *sql.Tx, mealID int64, items []ItemInput) error {
	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO meal_items (meal_id, food_id, grams) VALUES (?, ?, ?)`,
			mealID, item.FoodID, item.Grams)
		if err != nil {
			return fmt.Errorf("insert meal item: %w", err)
		}
	}
	return nil
}

// Create inserts a meal with its items atomically and returns the meal ID.
func (r *Repository) Create(ctx context.Context, userID int64, input Input) (int64, error) {
	var mealID int64
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO meals (user_id, eaten_on, meal_type) VALUES (?, ?, ?)`,
			userID, input.Date, input.Type)
		if err != nil {
			return fmt.Errorf("insert meal: %w", err)
		}
		if mealID, err = result.LastInsertId(); err != nil {
			return fmt.Errorf("read meal id: %w", err)
		}
		return insertItems(ctx, tx, mealID, input.Items)
	})
	return mealID, err
}

// Replace overwrites a meal of userID and all of its items atomically.
func (r *Repository) Replace(ctx context.Context, userID, id int64, input Input) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE meals SET eaten_on = ?, meal_type = ? WHERE id = ? AND user_id = ?`,
			input.Date, input.Type, id, userID)
		if err != nil {
			return fmt.Errorf("update meal: %w", err)
		}
		if err := requireAffected(result); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM meal_items WHERE meal_id = ?`, id); err != nil {
			return fmt.Errorf("delete meal items: %w", err)
		}
		return insertItems(ctx, tx, id, input.Items)
	})
}

// Delete removes a meal of userID; its items go with it by cascade.
func (r *Repository) Delete(ctx context.Context, userID, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM meals WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete meal: %w", err)
	}
	return requireAffected(result)
}

// Get returns the meal header with id if it belongs to userID.
func (r *Repository) Get(ctx context.Context, userID, id int64) (Meal, error) {
	found := Meal{ID: id}
	err := r.db.QueryRowContext(ctx,
		`SELECT eaten_on, meal_type FROM meals WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&found.Date, &found.Type)
	if errors.Is(err, sql.ErrNoRows) {
		return Meal{}, domain.ErrNotFound
	}
	if err != nil {
		return Meal{}, fmt.Errorf("select meal: %w", err)
	}
	return found, nil
}

// List returns one page of userID's meal headers, newest first, and the total.
func (r *Repository) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Meal, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM meals m WHERE `+dateFilter, dateArgs(userID, dates)...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count meals: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT m.id, m.eaten_on, m.meal_type FROM meals m WHERE `+dateFilter+`
		ORDER BY m.eaten_on DESC, m.id DESC LIMIT ? OFFSET ?`,
		append(dateArgs(userID, dates), page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("select meals: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	meals := make([]Meal, 0, page.Limit)
	for rows.Next() {
		var m Meal
		if err := rows.Scan(&m.ID, &m.Date, &m.Type); err != nil {
			return nil, 0, fmt.Errorf("scan meal: %w", err)
		}
		meals = append(meals, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate meals: %w", err)
	}
	return meals, total, nil
}

// itemsOfMeals returns the items of the given meals in insertion order.
func (r *Repository) itemsOfMeals(ctx context.Context, mealIDs []int64) ([]datedItem, error) {
	if len(mealIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(mealIDs)), ",")
	args := make([]any, 0, len(mealIDs))
	for _, id := range mealIDs {
		args = append(args, id)
	}
	return r.queryItems(ctx, `mi.meal_id IN (`+placeholders+`)`, args)
}

// itemsInRange returns every item userID ate within dates.
func (r *Repository) itemsInRange(ctx context.Context, userID int64, dates domain.DateRange) ([]datedItem, error) {
	return r.queryItems(ctx, dateFilter, dateArgs(userID, dates))
}

func (r *Repository) queryItems(ctx context.Context, filter string, args []any) ([]datedItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.eaten_on, mi.meal_id, mi.food_id, f.name, mi.grams,
			f.kcal_per_100g, f.protein_g_per_100g, f.carbs_g_per_100g, f.fat_g_per_100g
		FROM meal_items mi
		JOIN meals m ON m.id = mi.meal_id
		JOIN foods f ON f.id = mi.food_id
		WHERE `+filter+` ORDER BY mi.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("select meal items: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below reports the real failure.

	var items []datedItem
	for rows.Next() {
		var d datedItem
		err := rows.Scan(&d.Date, &d.MealID, &d.Item.FoodID, &d.Item.FoodName, &d.Item.Grams,
			&d.Per100g.Kcal, &d.Per100g.ProteinG, &d.Per100g.CarbsG, &d.Per100g.FatG)
		if err != nil {
			return nil, fmt.Errorf("scan meal item: %w", err)
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meal items: %w", err)
	}
	return items, nil
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
