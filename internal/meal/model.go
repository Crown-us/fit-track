// Package meal logs meals and computes their calories and macronutrients
// from the food table.
package meal

import "github.com/Crown-us/fit-track/internal/domain"

// gramsPerPortion is the reference portion the food table is expressed in.
const gramsPerPortion = 100.0

// Type is the kind of meal.
type Type string

// Supported meal types.
const (
	TypeBreakfast Type = "breakfast"
	TypeLunch     Type = "lunch"
	TypeDinner    Type = "dinner"
	TypeSnack     Type = "snack"
)

// Nutrition is an amount of energy and macronutrients.
type Nutrition struct {
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// add returns the sum of n and other.
func (n Nutrition) add(other Nutrition) Nutrition {
	return Nutrition{
		Kcal:     n.Kcal + other.Kcal,
		ProteinG: n.ProteinG + other.ProteinG,
		CarbsG:   n.CarbsG + other.CarbsG,
		FatG:     n.FatG + other.FatG,
	}
}

// rounded returns n rounded to the API's display precision.
func (n Nutrition) rounded() Nutrition {
	return Nutrition{
		Kcal:     domain.Round1(n.Kcal),
		ProteinG: domain.Round1(n.ProteinG),
		CarbsG:   domain.Round1(n.CarbsG),
		FatG:     domain.Round1(n.FatG),
	}
}

// ForGrams scales per-100 g values to the eaten amount.
func ForGrams(per100g Nutrition, grams float64) Nutrition {
	factor := grams / gramsPerPortion
	return Nutrition{
		Kcal:     per100g.Kcal * factor,
		ProteinG: per100g.ProteinG * factor,
		CarbsG:   per100g.CarbsG * factor,
		FatG:     per100g.FatG * factor,
	}
}

// Item is one food eaten as part of a meal.
type Item struct {
	FoodID    int64     `json:"food_id"`
	FoodName  string    `json:"food_name"`
	Grams     float64   `json:"grams"`
	Nutrition Nutrition `json:"nutrition"`
}

// Meal is one logged meal with its computed totals.
type Meal struct {
	ID     int64     `json:"id"`
	Date   string    `json:"date"`
	Type   Type      `json:"meal_type"`
	Items  []Item    `json:"items"`
	Totals Nutrition `json:"totals"`
}

// ItemInput references a food and the amount eaten.
type ItemInput struct {
	FoodID int64   `json:"food_id"`
	Grams  float64 `json:"grams"`
}

// Input is the payload for logging or replacing a meal.
type Input struct {
	Date  string      `json:"date"`
	Type  Type        `json:"meal_type"`
	Items []ItemInput `json:"items"`
}

// DayTotals aggregates the meals of one calendar day.
type DayTotals struct {
	Nutrition Nutrition
	MealCount int
}
