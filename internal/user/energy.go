package user

import "time"

// Mifflin-St Jeor coefficients (Mifflin MD, St Jeor ST, et al. "A new
// predictive equation for resting energy expenditure in healthy individuals."
// Am J Clin Nutr. 1990;51(2):241-247):
//
//	BMR = 10*weight(kg) + 6.25*height(cm) - 5*age(years) + s
//
// where s is +5 for males and -161 for females.
const (
	mifflinWeightCoef   = 10.0
	mifflinHeightCoef   = 6.25
	mifflinAgeCoef      = 5.0
	mifflinMaleOffset   = 5.0
	mifflinFemaleOffset = -161.0
)

const (
	// MaxDailyDeficitKcal caps the deficit for the "lose" goal. 500 kcal/day
	// is the commonly recommended moderate deficit (about 0.5 kg per week).
	MaxDailyDeficitKcal = 500.0
	// DailySurplusKcal is the surplus added for the "gain" goal; kept modest
	// so the gain is gradual.
	DailySurplusKcal = 300.0
)

// activityMultipliers are the standard physical-activity factors applied to
// BMR to estimate total daily energy expenditure.
var activityMultipliers = map[ActivityLevel]float64{
	ActivitySedentary:  1.2,
	ActivityLight:      1.375,
	ActivityModerate:   1.55,
	ActivityActive:     1.725,
	ActivityVeryActive: 1.9,
}

// IsValidActivityLevel reports whether level has a known multiplier.
func IsValidActivityLevel(level ActivityLevel) bool {
	_, ok := activityMultipliers[level]
	return ok
}

// BMR estimates basal metabolic rate in kcal/day with Mifflin-St Jeor.
func BMR(sex Sex, weightKg, heightCm float64, ageYears int) float64 {
	base := mifflinWeightCoef*weightKg + mifflinHeightCoef*heightCm - mifflinAgeCoef*float64(ageYears)
	if sex == SexMale {
		return base + mifflinMaleOffset
	}
	return base + mifflinFemaleOffset
}

// TDEE estimates total daily energy expenditure in kcal/day.
func TDEE(bmr float64, level ActivityLevel) float64 {
	return bmr * activityMultipliers[level]
}

// DailyTarget returns the daily calorie target for goal. The "lose" target
// never drops below BMR, so the API cannot recommend an extreme diet even
// when TDEE is close to BMR.
func DailyTarget(bmr, tdee float64, goal Goal) float64 {
	switch goal {
	case GoalLose:
		return max(tdee-MaxDailyDeficitKcal, bmr)
	case GoalGain:
		return tdee + DailySurplusKcal
	default:
		return tdee
	}
}

// AgeOn returns the age in completed years on the given day.
func AgeOn(birth, on time.Time) int {
	age := on.Year() - birth.Year()
	hasHadBirthday := on.Month() > birth.Month() ||
		(on.Month() == birth.Month() && on.Day() >= birth.Day())
	if !hasHadBirthday {
		age--
	}
	return age
}
