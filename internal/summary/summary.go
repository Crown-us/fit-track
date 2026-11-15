// Package summary combines meals, workouts and the profile target into daily
// and weekly overviews.
package summary

import (
	"context"
	"errors"
	"time"

	"github.com/Crown-us/fit-track/internal/domain"
	"github.com/Crown-us/fit-track/internal/meal"
	"github.com/Crown-us/fit-track/internal/user"
	"github.com/Crown-us/fit-track/internal/workout"
)

// DaysInWeek is the length of the weekly window.
const DaysInWeek = 7

// Day is the energy balance of one calendar day.
type Day struct {
	Date           string  `json:"date"`
	CaloriesIn     float64 `json:"calories_in"`
	CaloriesBurned float64 `json:"calories_burned"`
	ProteinG       float64 `json:"protein_g"`
	CarbsG         float64 `json:"carbs_g"`
	FatG           float64 `json:"fat_g"`
	MealCount      int     `json:"meal_count"`
	WorkoutCount   int     `json:"workout_count"`
	// WeightKg and TargetKcal are the weight that applied on this day and
	// the calorie target derived from it.
	WeightKg   float64 `json:"weight_kg"`
	TargetKcal float64 `json:"target_kcal"`
	// IsElapsed is false for days after today, which cannot have logs yet
	// and must not be read as "nothing logged".
	IsElapsed bool `json:"is_elapsed"`
}

// NetKcal is the intake left after subtracting exercise.
func (d Day) NetKcal() float64 {
	return d.CaloriesIn - d.CaloriesBurned
}

// Daily is a day's balance against the calorie target.
type Daily struct {
	Day
	RemainingKcal float64 `json:"remaining_kcal"`
}

// Weekly is a 7-day window with the context the coach needs. TargetKcal and
// WeightKg are those of the most recent elapsed day in the window.
type Weekly struct {
	WeekStart           string  `json:"week_start"`
	WeekEnd             string  `json:"week_end"`
	TargetKcal          float64 `json:"target_kcal"`
	WeightKg            float64 `json:"weight_kg"`
	WorkoutGoal         int     `json:"workout_goal"`
	WorkoutCount        int     `json:"workout_count"`
	TotalCaloriesIn     float64 `json:"total_calories_in"`
	TotalCaloriesBurned float64 `json:"total_calories_burned"`
	Days                []Day   `json:"days"`
}

// ProfileReader provides the calorie target and body data.
type ProfileReader interface {
	Profile(ctx context.Context, userID int64) (user.ProfileView, error)
}

// MealTotals provides per-day nutrition totals.
type MealTotals interface {
	DailyTotals(ctx context.Context, userID int64, dates domain.DateRange) (map[string]meal.DayTotals, error)
}

// WorkoutTotals provides per-day exercise totals.
type WorkoutTotals interface {
	DailyTotals(ctx context.Context, userID int64, dates domain.DateRange) (map[string]workout.DayTotals, error)
}

// WeightReader provides the body weight that applied on a given day.
type WeightReader interface {
	WeightOn(ctx context.Context, userID int64, date string) (float64, error)
}

// Sources are the read-only collaborators summaries are built from.
type Sources struct {
	Profiles ProfileReader
	Meals    MealTotals
	Workouts WorkoutTotals
	Weights  WeightReader
}

// Service builds summaries.
type Service struct {
	sources Sources
	now     func() time.Time
}

// NewService wires the summary service.
func NewService(sources Sources, now func() time.Time) *Service {
	return &Service{sources: sources, now: now}
}

// Daily returns the balance of one day. Remaining calories follow the common
// tracker convention: target - eaten + burned by logged exercise.
func (s *Service) Daily(ctx context.Context, userID int64, date time.Time) (Daily, error) {
	profile, err := s.profile(ctx, userID)
	if err != nil {
		return Daily{}, err
	}
	days, err := s.days(ctx, userID, profile.Profile, date, 1)
	if err != nil {
		return Daily{}, err
	}
	day := days[0]
	return Daily{Day: day, RemainingKcal: domain.Round1(day.TargetKcal - day.NetKcal())}, nil
}

// Weekly returns the 7 days starting at weekStart.
func (s *Service) Weekly(ctx context.Context, userID int64, weekStart time.Time) (Weekly, error) {
	profile, err := s.profile(ctx, userID)
	if err != nil {
		return Weekly{}, err
	}
	days, err := s.days(ctx, userID, profile.Profile, weekStart, DaysInWeek)
	if err != nil {
		return Weekly{}, err
	}
	reference := latestElapsed(days)
	week := Weekly{
		WeekStart:   days[0].Date,
		WeekEnd:     days[len(days)-1].Date,
		TargetKcal:  reference.TargetKcal,
		WeightKg:    reference.WeightKg,
		WorkoutGoal: profile.WeeklyWorkoutGoal,
		Days:        days,
	}
	for _, day := range days {
		week.WorkoutCount += day.WorkoutCount
		week.TotalCaloriesIn += day.CaloriesIn
		week.TotalCaloriesBurned += day.CaloriesBurned
	}
	week.TotalCaloriesIn = domain.Round1(week.TotalCaloriesIn)
	week.TotalCaloriesBurned = domain.Round1(week.TotalCaloriesBurned)
	return week, nil
}

// CurrentWeekStart returns the Monday of the week containing today.
func (s *Service) CurrentWeekStart() time.Time {
	today := s.Today()
	daysSinceMonday := (int(today.Weekday()) + DaysInWeek - int(time.Monday)) % DaysInWeek
	return today.AddDate(0, 0, -daysSinceMonday)
}

// Today returns the current calendar day in the timezone of the injected
// clock, expressed as midnight UTC so it compares cleanly with parsed dates.
func (s *Service) Today() time.Time {
	now := s.now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) profile(ctx context.Context, userID int64) (user.ProfileView, error) {
	profile, err := s.sources.Profiles.Profile(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		// Without a profile there is no target to summarise against.
		return user.ProfileView{}, domain.ErrProfileRequired
	}
	return profile, err
}

// latestElapsed returns the most recent day that has already started, or the
// first day when the whole window lies in the future.
func latestElapsed(days []Day) Day {
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].IsElapsed {
			return days[i]
		}
	}
	return days[0]
}

// energyOn returns the weight and calorie target that applied on date.
func (s *Service) energyOn(ctx context.Context, userID int64, profile user.Profile, date time.Time) (weightKg, targetKcal float64, err error) {
	weightKg, err = s.sources.Weights.WeightOn(ctx, userID, date.Format(domain.DateLayout))
	if err != nil {
		return 0, 0, err
	}
	energy, err := user.EnergyFor(profile, weightKg, date)
	if err != nil {
		return 0, 0, err
	}
	return weightKg, domain.Round1(energy.TargetKcal), nil
}

// days returns count consecutive days starting at start, including days
// without any logs.
func (s *Service) days(ctx context.Context, userID int64, profile user.Profile, start time.Time, count int) ([]Day, error) {
	dates := domain.DateRange{
		From: start.Format(domain.DateLayout),
		To:   start.AddDate(0, 0, count-1).Format(domain.DateLayout),
	}
	mealTotals, err := s.sources.Meals.DailyTotals(ctx, userID, dates)
	if err != nil {
		return nil, err
	}
	workoutTotals, err := s.sources.Workouts.DailyTotals(ctx, userID, dates)
	if err != nil {
		return nil, err
	}
	today := s.Today()
	days := make([]Day, 0, count)
	for offset := range count {
		date := start.AddDate(0, 0, offset)
		key := date.Format(domain.DateLayout)
		eaten, burned := mealTotals[key], workoutTotals[key]
		weightKg, targetKcal, err := s.energyOn(ctx, userID, profile, date)
		if err != nil {
			return nil, err
		}
		days = append(days, Day{
			Date:           key,
			CaloriesIn:     domain.Round1(eaten.Nutrition.Kcal),
			CaloriesBurned: domain.Round1(burned.CaloriesBurned),
			ProteinG:       domain.Round1(eaten.Nutrition.ProteinG),
			CarbsG:         domain.Round1(eaten.Nutrition.CarbsG),
			FatG:           domain.Round1(eaten.Nutrition.FatG),
			MealCount:      eaten.MealCount,
			WorkoutCount:   burned.WorkoutCount,
			WeightKg:       weightKg,
			TargetKcal:     targetKcal,
			IsElapsed:      !date.After(today),
		})
	}
	return days, nil
}
