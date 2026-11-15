package user

import (
	"math"
	"testing"
	"time"
)

const tolerance = 1e-9

func isClose(got, want float64) bool {
	return math.Abs(got-want) < tolerance
}

func TestBMR(t *testing.T) {
	tests := []struct {
		name     string
		sex      Sex
		weightKg float64
		heightCm float64
		age      int
		want     float64
	}{
		// 10*70 + 6.25*175 - 5*30 + 5
		{name: "male", sex: SexMale, weightKg: 70, heightCm: 175, age: 30, want: 1648.75},
		// 10*60 + 6.25*165 - 5*25 - 161
		{name: "female", sex: SexFemale, weightKg: 60, heightCm: 165, age: 25, want: 1345.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BMR(tt.sex, tt.weightKg, tt.heightCm, tt.age); !isClose(got, tt.want) {
				t.Errorf("BMR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTDEE(t *testing.T) {
	const bmr = 1000.0
	tests := []struct {
		level ActivityLevel
		want  float64
	}{
		{ActivitySedentary, 1200},
		{ActivityLight, 1375},
		{ActivityModerate, 1550},
		{ActivityActive, 1725},
		{ActivityVeryActive, 1900},
	}
	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if got := TDEE(bmr, tt.level); !isClose(got, tt.want) {
				t.Errorf("TDEE() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDailyTarget(t *testing.T) {
	tests := []struct {
		name string
		bmr  float64
		tdee float64
		goal Goal
		want float64
	}{
		{name: "maintain equals tdee", bmr: 1500, tdee: 2300, goal: GoalMaintain, want: 2300},
		{name: "lose subtracts capped deficit", bmr: 1500, tdee: 2300, goal: GoalLose, want: 1800},
		{name: "lose never drops below bmr", bmr: 1500, tdee: 1800, goal: GoalLose, want: 1500},
		{name: "gain adds surplus", bmr: 1500, tdee: 2300, goal: GoalGain, want: 2600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DailyTarget(tt.bmr, tt.tdee, tt.goal); !isClose(got, tt.want) {
				t.Errorf("DailyTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgeOn(t *testing.T) {
	birth := time.Date(1996, time.June, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		on   time.Time
		want int
	}{
		{name: "day before birthday", on: time.Date(2026, time.June, 14, 0, 0, 0, 0, time.UTC), want: 29},
		{name: "on birthday", on: time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC), want: 30},
		{name: "later in year", on: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), want: 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AgeOn(birth, tt.on); got != tt.want {
				t.Errorf("AgeOn() = %d, want %d", got, tt.want)
			}
		})
	}
}
