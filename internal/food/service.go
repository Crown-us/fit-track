package food

import (
	"context"
	"strings"

	"fittrack/internal/domain"
)

const (
	maxNameLength = 100
	// maxKcalPer100g is the energy of pure fat (9 kcal/g); nothing edible
	// exceeds it.
	maxKcalPer100g = 900.0
	// gramsPerPortion is the reference portion: macros of 100 g of food
	// cannot add up to more than 100 g.
	gramsPerPortion = 100.0
)

// Service implements food lookup and custom-food management.
type Service struct {
	repo *Repository
}

// NewService wires the food service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns a page of foods visible to userID, filtered by name.
func (s *Service) List(ctx context.Context, userID int64, query string, page domain.Page) ([]Food, int, error) {
	return s.repo.List(ctx, userID, strings.TrimSpace(query), page)
}

// Get returns a food visible to userID.
func (s *Service) Get(ctx context.Context, userID, id int64) (Food, error) {
	return s.repo.Get(ctx, userID, id)
}

// Create adds a custom food for userID.
func (s *Service) Create(ctx context.Context, userID int64, input Input) (Food, error) {
	input.Name = strings.TrimSpace(input.Name)
	if err := validate(input); err != nil {
		return Food{}, err
	}
	return s.repo.Create(ctx, userID, input)
}

// Update replaces a custom food owned by userID.
func (s *Service) Update(ctx context.Context, userID, id int64, input Input) (Food, error) {
	input.Name = strings.TrimSpace(input.Name)
	if err := validate(input); err != nil {
		return Food{}, err
	}
	if err := s.requireOwned(ctx, userID, id); err != nil {
		return Food{}, err
	}
	if err := s.repo.Update(ctx, userID, id, input); err != nil {
		return Food{}, err
	}
	return s.repo.Get(ctx, userID, id)
}

// Delete removes a custom food owned by userID.
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	if err := s.requireOwned(ctx, userID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

// requireOwned distinguishes "not visible" (not found) from "visible but
// global" (forbidden) so seeded foods stay read-only with a clear error.
func (s *Service) requireOwned(ctx context.Context, userID, id int64) error {
	found, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if !found.IsCustom {
		return domain.ErrForbidden
	}
	return nil
}

func validate(input Input) error {
	var v domain.Validator
	v.Check(input.Name != "" && len(input.Name) <= maxNameLength, "name", "must be 1 to 100 characters")
	v.Check(input.KcalPer100g >= 0 && input.KcalPer100g <= maxKcalPer100g, "kcal_per_100g", "must be between 0 and 900")
	v.Check(input.ProteinGPer100g >= 0, "protein_g_per_100g", "must be 0 or greater")
	v.Check(input.CarbsGPer100g >= 0, "carbs_g_per_100g", "must be 0 or greater")
	v.Check(input.FatGPer100g >= 0, "fat_g_per_100g", "must be 0 or greater")
	macros := input.ProteinGPer100g + input.CarbsGPer100g + input.FatGPer100g
	v.Check(macros <= gramsPerPortion, "macros", "protein, carbs and fat together cannot exceed 100 g per 100 g")
	return v.Err()
}
