package coach

import (
	"fmt"
	"math"

	"github.com/Crown-us/fit-track/internal/summary"
)

// Rule identifiers returned to clients.
const (
	RuleOnTrack           = "on_track"
	RuleWorkoutsBelowGoal = "workouts_below_goal"
	RuleCaloriesOver      = "calories_over_target"
	RuleCaloriesUnder     = "calories_under_target"
	RuleNoRecentLogs      = "no_recent_logs"
	RuleLowProtein        = "low_protein"
	RuleSuddenChange      = "sudden_change"
)

// Rule thresholds. These are product heuristics chosen to keep the coach
// quiet on normal day-to-day variation; they are not clinical cut-offs.
const (
	// overTargetRatio and underTargetRatio define the band around the
	// target that still counts as "on target".
	overTargetRatio  = 1.10
	underTargetRatio = 0.80
	// minDeviationDays is how many days must fall outside the band before
	// it counts as "consistently" over or under.
	minDeviationDays = 3
	// minSilentDays is the trailing gap without any log that triggers a nudge.
	minSilentDays = 2
	// minProteinGPerKg is the adult RDA for protein (0.8 g per kg of body
	// weight per day); an average below it is flagged as very low.
	minProteinGPerKg = 0.8
	// maxDailySwingRatio is the intake change between two consecutive
	// logged days, relative to the target, that counts as a sudden change.
	maxDailySwingRatio = 0.40
)

// rule inspects a week and reports a message when it fires. Rules are pure:
// the same week always yields the same result.
type rule func(week summary.Weekly) (Message, bool)

var rules = []rule{
	noRecentLogs,
	workoutsBelowGoal,
	caloriesOverTarget,
	caloriesUnderTarget,
	lowProtein,
	suddenChange,
}

func workoutsBelowGoal(week summary.Weekly) (Message, bool) {
	if week.WorkoutGoal == 0 || week.WorkoutCount >= week.WorkoutGoal {
		return Message{}, false
	}
	text := fmt.Sprintf("Minggu ini baru %d dari %d sesi olahraga. "+
		"Coba tambah satu sesi ringan, misalnya jalan kaki 20 menit.", week.WorkoutCount, week.WorkoutGoal)
	return Message{Rule: RuleWorkoutsBelowGoal, Text: text}, true
}

func caloriesOverTarget(week summary.Weekly) (Message, bool) {
	days := countMealDays(week, func(day summary.Day) bool {
		return day.NetKcal() > day.TargetKcal*overTargetRatio
	})
	if days < minDeviationDays {
		return Message{}, false
	}
	text := fmt.Sprintf("Asupan kalori melebihi target pada %d hari. "+
		"Coba kurangi camilan atau minuman manis sedikit demi sedikit.", days)
	return Message{Rule: RuleCaloriesOver, Text: text}, true
}

func caloriesUnderTarget(week summary.Weekly) (Message, bool) {
	days := countMealDays(week, func(day summary.Day) bool {
		return day.NetKcal() < day.TargetKcal*underTargetRatio
	})
	if days < minDeviationDays {
		return Message{}, false
	}
	text := fmt.Sprintf("Asupan kalori jauh di bawah target pada %d hari. "+
		"Makan terlalu sedikit bisa bikin lemas, coba tambah porsi makan utama.", days)
	return Message{Rule: RuleCaloriesUnder, Text: text}, true
}

// noRecentLogs fires when the most recent elapsed days have no meal and no
// workout. Future days are skipped so a week in progress is judged fairly.
func noRecentLogs(week summary.Weekly) (Message, bool) {
	silentDays := 0
	for i := len(week.Days) - 1; i >= 0; i-- {
		day := week.Days[i]
		if !day.IsElapsed {
			continue
		}
		if day.MealCount > 0 || day.WorkoutCount > 0 {
			break
		}
		silentDays++
	}
	if silentDays < minSilentDays {
		return Message{}, false
	}
	text := fmt.Sprintf("Belum ada catatan selama %d hari terakhir. "+
		"Yuk mulai lagi dari satu catatan makan hari ini.", silentDays)
	return Message{Rule: RuleNoRecentLogs, Text: text}, true
}

func lowProtein(week summary.Weekly) (Message, bool) {
	totalProtein, mealDays := 0.0, 0
	for _, day := range week.Days {
		if day.MealCount > 0 {
			totalProtein += day.ProteinG
			mealDays++
		}
	}
	if mealDays == 0 {
		return Message{}, false
	}
	average := totalProtein / float64(mealDays)
	minimum := week.WeightKg * minProteinGPerKg
	if average >= minimum {
		return Message{}, false
	}
	text := fmt.Sprintf("Rata-rata protein %.0f g per hari, di bawah kisaran umum sekitar %.0f g. "+
		"Coba tambah lauk seperti tempe, tahu, telur, atau ikan.", average, minimum)
	return Message{Rule: RuleLowProtein, Text: text}, true
}

// suddenChange compares only consecutive calendar days that both have meals,
// so a day without logs is not mistaken for a day of eating nothing.
func suddenChange(week summary.Weekly) (Message, bool) {
	maxSwing := week.TargetKcal * maxDailySwingRatio
	for i := 1; i < len(week.Days); i++ {
		previous, current := week.Days[i-1], week.Days[i]
		if previous.MealCount == 0 || current.MealCount == 0 {
			continue
		}
		if math.Abs(current.CaloriesIn-previous.CaloriesIn) > maxSwing {
			return Message{
				Rule: RuleSuddenChange,
				Text: "Asupan kalori naik turun cukup jauh antarhari. " +
					"Pola makan yang lebih rata biasanya lebih mudah dijaga.",
			}, true
		}
	}
	return Message{}, false
}

// countMealDays counts days with at least one meal that satisfy matches.
// Days without meals are ignored because their intake is unknown, not zero.
func countMealDays(week summary.Weekly, matches func(summary.Day) bool) int {
	count := 0
	for _, day := range week.Days {
		if day.MealCount > 0 && matches(day) {
			count++
		}
	}
	return count
}
