// Package user manages accounts, profiles and the energy-need calculations
// derived from a profile.
package user

// Sex is the biological sex used by the Mifflin-St Jeor equation.
type Sex string

// Supported sexes.
const (
	SexMale   Sex = "male"
	SexFemale Sex = "female"
)

// ActivityLevel describes habitual daily activity.
type ActivityLevel string

// Supported activity levels.
const (
	ActivitySedentary  ActivityLevel = "sedentary"
	ActivityLight      ActivityLevel = "light"
	ActivityModerate   ActivityLevel = "moderate"
	ActivityActive     ActivityLevel = "active"
	ActivityVeryActive ActivityLevel = "very_active"
)

// Goal is what the user wants to do with their weight.
type Goal string

// Supported goals.
const (
	GoalLose     Goal = "lose"
	GoalMaintain Goal = "maintain"
	GoalGain     Goal = "gain"
)

// User is a registered account.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

// Profile is the body data a user maintains.
type Profile struct {
	UserID            int64         `json:"-"`
	Sex               Sex           `json:"sex"`
	BirthDate         string        `json:"birth_date"`
	HeightCm          float64       `json:"height_cm"`
	WeightKg          float64       `json:"weight_kg"`
	ActivityLevel     ActivityLevel `json:"activity_level"`
	Goal              Goal          `json:"goal"`
	WeeklyWorkoutGoal int           `json:"weekly_workout_goal"`
}

// ProfileView is a profile together with the energy needs computed from it.
type ProfileView struct {
	Profile
	BMRKcal         float64 `json:"bmr_kcal"`
	TDEEKcal        float64 `json:"tdee_kcal"`
	DailyTargetKcal float64 `json:"daily_target_kcal"`
}

// ProfileInput is the payload for creating or replacing a profile.
// WeeklyWorkoutGoal is a pointer so an omitted value can take the default.
type ProfileInput struct {
	Sex               Sex           `json:"sex"`
	BirthDate         string        `json:"birth_date"`
	HeightCm          float64       `json:"height_cm"`
	WeightKg          float64       `json:"weight_kg"`
	ActivityLevel     ActivityLevel `json:"activity_level"`
	Goal              Goal          `json:"goal"`
	WeeklyWorkoutGoal *int          `json:"weekly_workout_goal"`
}
