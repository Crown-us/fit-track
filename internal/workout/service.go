package workout

import (
	"context"
	"errors"

	"fittrack/internal/domain"
	"fittrack/internal/user"
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

// ProfileReader provides the body weight the calorie estimate depends on.
type ProfileReader interface {
	Profile(ctx context.Context, userID int64) (user.ProfileView, error)
}

// Service implements workout logging.
type Service struct {
	repo     *Repository
	profiles ProfileReader
}

// NewService wires the workout service.
func NewService(repo *Repository, profiles ProfileReader) *Service {
	return &Service{repo: repo, profiles: profiles}
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

// build validates input and computes calories with the user's current weight.
// The estimate is stored, so later weight changes do not rewrite history.
func (s *Service) build(ctx context.Context, userID int64, input Input) (Workout, error) {
	if err := validate(input); err != nil {
		return Workout{}, err
	}
	profile, err := s.profiles.Profile(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return Workout{}, domain.ErrProfileRequired
	}
	if err != nil {
		return Workout{}, err
	}
	met, _ := MET(input.Activity)
	return Workout{
		Activity:       input.Activity,
		DurationMin:    input.DurationMin,
		DistanceKm:     input.DistanceKm,
		Date:           input.Date,
		CaloriesBurned: domain.Round1(CaloriesBurned(met, profile.WeightKg, input.DurationMin)),
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
