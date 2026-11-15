// Package food manages the food composition table: seeded global foods and
// the custom foods each user adds.
package food

// Verification statuses of a food's nutrition values.
const (
	StatusVerified          = "verified"
	StatusNeedsVerification = "needs_verification"
	StatusUserProvided      = "user_provided"
)

// sourceUser labels foods entered by a user rather than seeded.
const sourceUser = "user"

// Food holds nutrition values per 100 g of edible portion.
type Food struct {
	ID                 int64   `json:"id"`
	OwnerID            *int64  `json:"-"`
	Name               string  `json:"name"`
	KcalPer100g        float64 `json:"kcal_per_100g"`
	ProteinGPer100g    float64 `json:"protein_g_per_100g"`
	CarbsGPer100g      float64 `json:"carbs_g_per_100g"`
	FatGPer100g        float64 `json:"fat_g_per_100g"`
	Source             string  `json:"source"`
	VerificationStatus string  `json:"verification_status"`
	IsCustom           bool    `json:"is_custom"`
}

// Input is the payload for creating or replacing a custom food.
type Input struct {
	Name            string  `json:"name"`
	KcalPer100g     float64 `json:"kcal_per_100g"`
	ProteinGPer100g float64 `json:"protein_g_per_100g"`
	CarbsGPer100g   float64 `json:"carbs_g_per_100g"`
	FatGPer100g     float64 `json:"fat_g_per_100g"`
}
