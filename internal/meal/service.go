package meal

import (
	"context"
	"errors"
	"fmt"

	"fittrack/internal/domain"
	"fittrack/internal/food"
)

const (
	maxItemsPerMeal = 50
	// maxGramsPerItem rejects unit mix-ups such as milligrams.
	maxGramsPerItem = 5000.0
)

// FoodReader confirms that a food exists and is visible to the user.
type FoodReader interface {
	Get(ctx context.Context, userID, id int64) (food.Food, error)
}

// Service implements meal logging.
type Service struct {
	repo  *Repository
	foods FoodReader
}

// NewService wires the meal service.
func NewService(repo *Repository, foods FoodReader) *Service {
	return &Service{repo: repo, foods: foods}
}

// List returns a page of userID's meals within dates, with items and totals.
func (s *Service) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Meal, int, error) {
	meals, total, err := s.repo.List(ctx, userID, dates, page)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachItems(ctx, meals); err != nil {
		return nil, 0, err
	}
	return meals, total, nil
}

// Get returns one meal of userID with items and totals.
func (s *Service) Get(ctx context.Context, userID, id int64) (Meal, error) {
	found, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return Meal{}, err
	}
	meals := []Meal{found}
	if err := s.attachItems(ctx, meals); err != nil {
		return Meal{}, err
	}
	return meals[0], nil
}

// Create logs a meal for userID.
func (s *Service) Create(ctx context.Context, userID int64, input Input) (Meal, error) {
	if err := s.validate(ctx, userID, input); err != nil {
		return Meal{}, err
	}
	id, err := s.repo.Create(ctx, userID, input)
	if err != nil {
		return Meal{}, err
	}
	return s.Get(ctx, userID, id)
}

// Update replaces a meal of userID, including all of its items.
func (s *Service) Update(ctx context.Context, userID, id int64, input Input) (Meal, error) {
	if err := s.validate(ctx, userID, input); err != nil {
		return Meal{}, err
	}
	if err := s.repo.Replace(ctx, userID, id, input); err != nil {
		return Meal{}, err
	}
	return s.Get(ctx, userID, id)
}

// Delete removes one meal of userID.
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	return s.repo.Delete(ctx, userID, id)
}

// DailyTotals returns the nutrition and meal count of each day in dates that
// has at least one meal.
func (s *Service) DailyTotals(ctx context.Context, userID int64, dates domain.DateRange) (map[string]DayTotals, error) {
	items, err := s.repo.itemsInRange(ctx, userID, dates)
	if err != nil {
		return nil, err
	}
	totals := make(map[string]DayTotals)
	mealsSeen := make(map[int64]bool)
	for _, d := range items {
		day := totals[d.Date]
		day.Nutrition = day.Nutrition.add(ForGrams(d.Per100g, d.Item.Grams))
		if !mealsSeen[d.MealID] {
			mealsSeen[d.MealID] = true
			day.MealCount++
		}
		totals[d.Date] = day
	}
	return totals, nil
}

// attachItems fills Items and Totals of every meal with one query.
func (s *Service) attachItems(ctx context.Context, meals []Meal) error {
	ids := make([]int64, 0, len(meals))
	indexByID := make(map[int64]int, len(meals))
	for i := range meals {
		ids = append(ids, meals[i].ID)
		indexByID[meals[i].ID] = i
		meals[i].Items = []Item{}
	}
	items, err := s.repo.itemsOfMeals(ctx, ids)
	if err != nil {
		return err
	}
	exact := make([]Nutrition, len(meals))
	for _, d := range items {
		i := indexByID[d.MealID]
		nutrition := ForGrams(d.Per100g, d.Item.Grams)
		d.Item.Nutrition = nutrition.rounded()
		meals[i].Items = append(meals[i].Items, d.Item)
		// Totals are summed before rounding so rounding errors do not add up.
		exact[i] = exact[i].add(nutrition)
	}
	for i := range meals {
		meals[i].Totals = exact[i].rounded()
	}
	return nil
}

func (s *Service) validate(ctx context.Context, userID int64, input Input) error {
	var v domain.Validator
	v.Check(domain.IsDate(input.Date), "date", "must be a date in YYYY-MM-DD format")
	v.Check(isValidType(input.Type), "meal_type", "must be breakfast, lunch, dinner or snack")
	v.Check(len(input.Items) >= 1 && len(input.Items) <= maxItemsPerMeal, "items", "must contain 1 to 50 items")
	if len(input.Items) > maxItemsPerMeal {
		return v.Err()
	}
	for i, item := range input.Items {
		v.Check(item.Grams > 0 && item.Grams <= maxGramsPerItem,
			fmt.Sprintf("items[%d].grams", i), "must be greater than 0 and at most 5000")
		_, err := s.foods.Get(ctx, userID, item.FoodID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		v.Check(err == nil, fmt.Sprintf("items[%d].food_id", i), "food does not exist")
	}
	return v.Err()
}

func isValidType(mealType Type) bool {
	switch mealType {
	case TypeBreakfast, TypeLunch, TypeDinner, TypeSnack:
		return true
	default:
		return false
	}
}
