// Package summary combines meals, workouts and the profile target into daily
// and weekly overviews.
package summary

import (
	"context"
	"errors"
	"time"

	"fittrack/internal/domain"
	"fittrack/internal/meal"
	"fittrack/internal/user"
	"fittrack/internal/workout"
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
	TargetKcal    float64 `json:"target_kcal"`
	RemainingKcal float64 `json:"remaining_kcal"`
}

// Weekly is a 7-day window with the context the coach needs.
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

// Service builds summaries.
type Service struct {
	profiles ProfileReader
	meals    MealTotals
	workouts WorkoutTotals
	now      func() time.Time
}

// NewService wires the summary service.
func NewService(profiles ProfileReader, meals MealTotals, workouts WorkoutTotals, now func() time.Time) *Service {
	return &Service{profiles: profiles, meals: meals, workouts: workouts, now: now}
}

// Daily returns the balance of one day. Remaining calories follow the common
// tracker convention: target - eaten + burned by logged exercise.
func (s *Service) Daily(ctx context.Context, userID int64, date time.Time) (Daily, error) {
	profile, err := s.profile(ctx, userID)
	if err != nil {
		return Daily{}, err
	}
	days, err := s.days(ctx, userID, date, 1)
	if err != nil {
		return Daily{}, err
	}
	day := days[0]
	return Daily{
		Day:           day,
		TargetKcal:    profile.DailyTargetKcal,
		RemainingKcal: domain.Round1(profile.DailyTargetKcal - day.NetKcal()),
	}, nil
}

// Weekly returns the 7 days starting at weekStart.
func (s *Service) Weekly(ctx context.Context, userID int64, weekStart time.Time) (Weekly, error) {
	profile, err := s.profile(ctx, userID)
	if err != nil {
		return Weekly{}, err
	}
	days, err := s.days(ctx, userID, weekStart, DaysInWeek)
	if err != nil {
		return Weekly{}, err
	}
	week := Weekly{
		WeekStart:   days[0].Date,
		WeekEnd:     days[len(days)-1].Date,
		TargetKcal:  profile.DailyTargetKcal,
		WeightKg:    profile.WeightKg,
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

// Today returns the current calendar day in UTC.
func (s *Service) Today() time.Time {
	now := s.now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) profile(ctx context.Context, userID int64) (user.ProfileView, error) {
	profile, err := s.profiles.Profile(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		// Without a profile there is no target to summarise against.
		return user.ProfileView{}, domain.ErrProfileRequired
	}
	return profile, err
}

// days returns count consecutive days starting at start, including days
// without any logs.
func (s *Service) days(ctx context.Context, userID int64, start time.Time, count int) ([]Day, error) {
	dates := domain.DateRange{
		From: start.Format(domain.DateLayout),
		To:   start.AddDate(0, 0, count-1).Format(domain.DateLayout),
	}
	mealTotals, err := s.meals.DailyTotals(ctx, userID, dates)
	if err != nil {
		return nil, err
	}
	workoutTotals, err := s.workouts.DailyTotals(ctx, userID, dates)
	if err != nil {
		return nil, err
	}
	today := s.Today()
	days := make([]Day, 0, count)
	for offset := range count {
		date := start.AddDate(0, 0, offset)
		key := date.Format(domain.DateLayout)
		eaten, burned := mealTotals[key], workoutTotals[key]
		days = append(days, Day{
			Date:           key,
			CaloriesIn:     domain.Round1(eaten.Nutrition.Kcal),
			CaloriesBurned: domain.Round1(burned.CaloriesBurned),
			ProteinG:       domain.Round1(eaten.Nutrition.ProteinG),
			CarbsG:         domain.Round1(eaten.Nutrition.CarbsG),
			FatG:           domain.Round1(eaten.Nutrition.FatG),
			MealCount:      eaten.MealCount,
			WorkoutCount:   burned.WorkoutCount,
			IsElapsed:      !date.After(today),
		})
	}
	return days, nil
}
