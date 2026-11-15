package coach

import (
	"context"
	"slices"
	"testing"

	"github.com/Crown-us/fit-track/internal/summary"
)

const (
	testTarget = 2000.0
	testWeight = 60.0 // protein minimum = 48 g/day
)

// day builds an elapsed day with one meal of the given intake and protein.
func day(caloriesIn, proteinG float64) summary.Day {
	return summary.Day{CaloriesIn: caloriesIn, ProteinG: proteinG, MealCount: 1, IsElapsed: true, TargetKcal: testTarget}
}

func emptyDay() summary.Day {
	return summary.Day{IsElapsed: true}
}

func futureDay() summary.Day {
	return summary.Day{}
}

func workoutDay() summary.Day {
	return summary.Day{WorkoutCount: 1, CaloriesBurned: 200, IsElapsed: true}
}

// onTargetWeek is a week in which no rule should fire.
func onTargetWeek() summary.Weekly {
	days := make([]summary.Day, 0, summary.DaysInWeek)
	for range summary.DaysInWeek {
		days = append(days, day(testTarget, 80))
	}
	return summary.Weekly{TargetKcal: testTarget, WeightKg: testWeight, WorkoutGoal: 3, WorkoutCount: 3, Days: days}
}

func withDays(days ...summary.Day) summary.Weekly {
	week := onTargetWeek()
	week.Days = days
	return week
}

func firedRules(messages []Message) []string {
	fired := make([]string, 0, len(messages))
	for _, message := range messages {
		fired = append(fired, message.Rule)
	}
	return fired
}

func TestEvaluate(t *testing.T) {
	normal := day(testTarget, 80)
	tests := []struct {
		name string
		week summary.Weekly
		want []string
	}{
		{name: "on track", week: onTargetWeek(), want: []string{RuleOnTrack}},
		{
			name: "workouts below goal",
			week: func() summary.Weekly { w := onTargetWeek(); w.WorkoutCount = 1; return w }(),
			want: []string{RuleWorkoutsBelowGoal},
		},
		{
			name: "no workout goal never nags",
			week: func() summary.Weekly { w := onTargetWeek(); w.WorkoutGoal, w.WorkoutCount = 0, 0; return w }(),
			want: []string{RuleOnTrack},
		},
		{
			name: "over target on three days",
			week: withDays(day(2300, 80), day(2300, 80), day(2300, 80), normal, normal, normal, normal),
			want: []string{RuleCaloriesOver},
		},
		{
			name: "over target on two days is tolerated",
			week: withDays(day(2300, 80), day(2300, 80), normal, normal, normal, normal, normal),
			want: []string{RuleOnTrack},
		},
		{
			name: "exactly at the upper band is tolerated",
			week: withDays(day(2200, 80), day(2200, 80), day(2200, 80), normal, normal, normal, normal),
			want: []string{RuleOnTrack},
		},
		{
			name: "under target on three days",
			week: withDays(day(1500, 80), day(1500, 80), day(1500, 80), normal, normal, normal, normal),
			want: []string{RuleCaloriesUnder},
		},
		{
			name: "two trailing silent days",
			week: withDays(normal, normal, normal, normal, normal, emptyDay(), emptyDay()),
			want: []string{RuleNoRecentLogs},
		},
		{
			name: "one trailing silent day is tolerated",
			week: withDays(normal, normal, normal, normal, normal, normal, emptyDay()),
			want: []string{RuleOnTrack},
		},
		{
			name: "a workout counts as a log",
			week: withDays(normal, normal, normal, normal, normal, workoutDay(), workoutDay()),
			want: []string{RuleOnTrack},
		},
		{
			name: "future days are not silent days",
			week: withDays(normal, normal, normal, futureDay(), futureDay(), futureDay(), futureDay()),
			want: []string{RuleOnTrack},
		},
		{
			name: "silent days before future days still count",
			week: withDays(normal, emptyDay(), emptyDay(), futureDay(), futureDay(), futureDay(), futureDay()),
			want: []string{RuleNoRecentLogs},
		},
		{
			// (5*30 + 80) / 6 logged days = 38 g, below the 48 g minimum.
			name: "low protein averaged over logged days only",
			week: withDays(day(testTarget, 30), day(testTarget, 30), day(testTarget, 30),
				day(testTarget, 30), emptyDay(), day(testTarget, 30), normal),
			want: []string{RuleLowProtein},
		},
		{
			name: "sudden change between consecutive days",
			week: withDays(normal, day(2900, 80), normal, normal, normal, normal, normal),
			want: []string{RuleSuddenChange},
		},
		{
			name: "a gap day does not count as a sudden change",
			week: withDays(normal, emptyDay(), day(2900, 80), day(2900, 80), day(2200, 80), normal, normal),
			want: []string{RuleOnTrack},
		},
		{
			name: "several rules fire together in rule order",
			week: func() summary.Weekly {
				w := withDays(day(2300, 20), day(2300, 20), day(2300, 20), normal, normal, emptyDay(), emptyDay())
				w.WorkoutCount = 0
				return w
			}(),
			want: []string{RuleNoRecentLogs, RuleWorkoutsBelowGoal, RuleCaloriesOver, RuleLowProtein},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firedRules(Evaluate(tt.week)); !slices.Equal(got, tt.want) {
				t.Errorf("Evaluate() fired %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetCaloriesUseExercise(t *testing.T) {
	// 2300 eaten minus 300 burned is on target, so the over rule stays quiet.
	active := day(2300, 80)
	active.CaloriesBurned, active.WorkoutCount = 300, 1
	normal := day(testTarget, 80)
	week := withDays(active, active, active, normal, normal, normal, normal)
	if got := firedRules(Evaluate(week)); !slices.Equal(got, []string{RuleOnTrack}) {
		t.Errorf("Evaluate() fired %v, want on_track", got)
	}
}

func TestRuleBasedAdvise(t *testing.T) {
	week := onTargetWeek()
	week.WeekStart, week.WeekEnd = "2026-11-09", "2026-11-15"
	var coach Coach = NewRuleBased()
	advice, err := coach.Advise(context.Background(), week)
	if err != nil {
		t.Fatalf("Advise() error = %v", err)
	}
	if advice.Disclaimer != Disclaimer || advice.WeekStart != week.WeekStart || advice.WeekEnd != week.WeekEnd {
		t.Errorf("Advise() = %+v, missing disclaimer or week bounds", advice)
	}
	if len(advice.Messages) == 0 {
		t.Error("Advise() returned no messages")
	}
}
