package weight

import (
	"context"
	"errors"
	"time"

	"github.com/Crown-us/fit-track/internal/domain"
	"github.com/Crown-us/fit-track/internal/user"
)

// Service implements weight logging.
type Service struct {
	repo *Repository
	now  func() time.Time
}

// NewService wires the weight service.
func NewService(repo *Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now}
}

// List returns a page of userID's weight entries within dates.
func (s *Service) List(ctx context.Context, userID int64, dates domain.DateRange, page domain.Page) ([]Entry, int, error) {
	return s.repo.List(ctx, userID, dates, page)
}

// Log records the weight of one day, replacing an earlier entry of that day.
func (s *Service) Log(ctx context.Context, userID int64, entry Entry) (Entry, error) {
	if err := s.validate(entry); err != nil {
		return Entry{}, err
	}
	if err := s.repo.Upsert(ctx, userID, entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Delete removes the entry of date. The last remaining entry cannot be
// deleted, because workouts and summaries need a weight to compute with.
func (s *Service) Delete(ctx context.Context, userID int64, date string) error {
	err := s.repo.Delete(ctx, userID, date)
	if errors.Is(err, ErrLastEntry) {
		var v domain.Validator
		v.Check(false, "date", "the only weight entry cannot be deleted; log another weight first")
		return v.Err()
	}
	return err
}

// WeightOn returns the weight that applies to userID on date. Saving a
// profile always logs a weight, so having no entries means the profile is
// still missing.
func (s *Service) WeightOn(ctx context.Context, userID int64, date string) (float64, error) {
	weightKg, err := s.repo.On(ctx, userID, date)
	if errors.Is(err, domain.ErrNotFound) {
		return 0, domain.ErrProfileRequired
	}
	return weightKg, err
}

func (s *Service) validate(entry Entry) error {
	var v domain.Validator
	date, err := domain.ParseDate(entry.Date)
	v.Check(err == nil, "date", "must be a date in YYYY-MM-DD format")
	if err == nil {
		// A future weight would silently become the "current" weight.
		today := s.now().Format(domain.DateLayout)
		v.Check(date.Format(domain.DateLayout) <= today, "date", "cannot be in the future")
	}
	v.Check(entry.WeightKg >= user.MinWeightKg && entry.WeightKg <= user.MaxWeightKg,
		"weight_kg", "must be between 20 and 500")
	return v.Err()
}
