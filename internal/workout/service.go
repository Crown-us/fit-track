package workout

import (
	"context"

	"github.com/Crown-us/fit-track/internal/domain"
)

const (
	// maxDurationMin is one full day; longer sessions are input mistakes.
	maxDurationMin = 1440.0
	maxDistanceKm  = 1000.0
)

// Input is the payload for logging or replacing a workout.
type Input struct {
	Activity    Activity `json:"activity"`
	DurationMin float64  `json:"duration_min"`
	DistanceKm  *float64 `json:"distance_km"`
	Date        string   `json:"date"`
}

// WeightReader provides the body weight that applied on a given day.
type WeightReader interface {
	WeightOn(ctx context.Context, userID int64, date string) (float64, error)
}

// Service implements workout logging.
type Service struct {
	repo    *Repository
	weights WeightReader
}

// NewService wires the workout service.
func NewService(repo *Repository, weights WeightReader) *Service {
	return &Service{repo: repo, weights: weights}
}

// List returns a page of userID's workouts within dates.
func (s *Service) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Workout, int, error) {
	return s.repo.List(ctx, userID, dates, page)
}

// Get returns one workout of userID.
func (s *Service) Get(ctx context.Context, userID, id int64) (Workout, error) {
	return s.repo.Get(ctx, userID, id)
}

// Create logs a workout and stores its calorie estimate.
func (s *Service) Create(ctx context.Context, userID int64, input Input) (Workout, error) {
	w, err := s.build(ctx, userID, input)
	if err != nil {
		return Workout{}, err
	}
	id, err := s.repo.Create(ctx, userID, w)
	if err != nil {
		return Workout{}, err
	}
	w.ID = id
	return w, nil
}

// Update replaces a workout and recomputes its calorie estimate.
func (s *Service) Update(ctx context.Context, userID, id int64, input Input) (Workout, error) {
	w, err := s.build(ctx, userID, input)
	if err != nil {
		return Workout{}, err
	}
	w.ID = id
	if err := s.repo.Update(ctx, userID, w); err != nil {
		return Workout{}, err
	}
	return w, nil
}

// Delete removes one workout of userID.
func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	return s.repo.Delete(ctx, userID, id)
}

// build validates input and computes calories with the weight the user had
// on the workout date. The estimate is stored, so later weight changes do not
// rewrite history.
func (s *Service) build(ctx context.Context, userID int64, input Input) (Workout, error) {
	if err := validate(input); err != nil {
		return Workout{}, err
	}
	weightKg, err := s.weights.WeightOn(ctx, userID, input.Date)
	if err != nil {
		return Workout{}, err
	}
	met, _ := MET(input.Activity)
	return Workout{
		Activity:       input.Activity,
		DurationMin:    input.DurationMin,
		DistanceKm:     input.DistanceKm,
		Date:           input.Date,
		CaloriesBurned: domain.Round1(CaloriesBurned(met, weightKg, input.DurationMin)),
	}, nil
}

func validate(input Input) error {
	var v domain.Validator
	_, isSupported := MET(input.Activity)
	v.Check(isSupported, "activity", "is not a supported activity; see GET /api/v1/activities")
	v.Check(input.DurationMin > 0 && input.DurationMin <= maxDurationMin, "duration_min", "must be between 0 and 1440")
	if distance := input.DistanceKm; distance != nil {
		v.Check(*distance >= 0 && *distance <= maxDistanceKm, "distance_km", "must be between 0 and 1000")
	}
	v.Check(domain.IsDate(input.Date), "date", "must be a date in YYYY-MM-DD format")
	return v.Err()
}
