package user

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/Crown-us/fit-track/internal/auth"
	"github.com/Crown-us/fit-track/internal/domain"
)

// Input bounds. They reject typos and unit mix-ups (for example height in
// metres), not unusual-but-real bodies.
const (
	minPasswordLength = 8
	// maxPasswordLength is bcrypt's hard input limit in bytes.
	maxPasswordLength = 72

	minAgeYears  = 13
	maxAgeYears  = 120
	minHeightCm  = 50.0
	maxHeightCm  = 280.0
	maxWorkouts  = 14
	maxEmailSize = 254

	// MinWeightKg and MaxWeightKg bound every weight the API accepts.
	MinWeightKg = 20.0
	MaxWeightKg = 500.0

	// DefaultWeeklyWorkoutGoal is used when the profile does not set one.
	DefaultWeeklyWorkoutGoal = 3
)

// Service implements registration, login and profile management.
type Service struct {
	repo   *Repository
	hasher auth.PasswordHasher
	tokens *auth.TokenManager
	now    func() time.Time
}

// NewService wires the user service.
func NewService(repo *Repository, hasher auth.PasswordHasher, tokens *auth.TokenManager, now func() time.Time) *Service {
	return &Service{repo: repo, hasher: hasher, tokens: tokens, now: now}
}

// Register creates an account and returns it with an access token.
func (s *Service) Register(ctx context.Context, email, password string) (User, string, error) {
	email = normalizeEmail(email)
	if err := validateCredentials(email, password); err != nil {
		return User{}, "", err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return User{}, "", err
	}
	created, err := s.repo.Create(ctx, email, hash)
	if err != nil {
		return User{}, "", err
	}
	token, err := s.tokens.Issue(created.ID)
	if err != nil {
		return User{}, "", err
	}
	return created, token, nil
}

// Login verifies credentials and returns an access token. Unknown email and
// wrong password produce the same error so accounts cannot be enumerated.
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	found, err := s.repo.FindByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, domain.ErrNotFound) {
		return "", domain.ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if !s.hasher.IsMatch(found.PasswordHash, password) {
		return "", domain.ErrInvalidCredentials
	}
	return s.tokens.Issue(found.ID)
}

// Profile returns the profile of userID with its computed energy needs.
func (s *Service) Profile(ctx context.Context, userID int64) (ProfileView, error) {
	profile, err := s.repo.Profile(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	return s.view(profile)
}

// UpdateProfile validates input and creates or replaces the profile.
func (s *Service) UpdateProfile(ctx context.Context, userID int64, input ProfileInput) (ProfileView, error) {
	if err := s.validateProfile(input); err != nil {
		return ProfileView{}, err
	}
	profile := Profile{
		UserID:            userID,
		Sex:               input.Sex,
		BirthDate:         input.BirthDate,
		HeightCm:          input.HeightCm,
		WeightKg:          input.WeightKg,
		ActivityLevel:     input.ActivityLevel,
		Goal:              input.Goal,
		WeeklyWorkoutGoal: DefaultWeeklyWorkoutGoal,
	}
	if input.WeeklyWorkoutGoal != nil {
		profile.WeeklyWorkoutGoal = *input.WeeklyWorkoutGoal
	}
	// The profile weight is also today's entry in the weight history.
	if err := s.repo.SaveProfile(ctx, profile, s.now().Format(domain.DateLayout)); err != nil {
		return ProfileView{}, err
	}
	return s.view(profile)
}

func (s *Service) view(profile Profile) (ProfileView, error) {
	energy, err := EnergyFor(profile, profile.WeightKg, s.now())
	if err != nil {
		return ProfileView{}, err
	}
	return ProfileView{
		Profile:         profile,
		BMRKcal:         domain.Round1(energy.BMRKcal),
		TDEEKcal:        domain.Round1(energy.TDEEKcal),
		DailyTargetKcal: domain.Round1(energy.TargetKcal),
	}, nil
}

func (s *Service) validateProfile(input ProfileInput) error {
	var v domain.Validator
	v.Check(input.Sex == SexMale || input.Sex == SexFemale, "sex", "must be male or female")
	birth, err := domain.ParseDate(input.BirthDate)
	v.Check(err == nil, "birth_date", "must be a date in YYYY-MM-DD format")
	if err == nil {
		age := AgeOn(birth, s.now())
		v.Check(age >= minAgeYears && age <= maxAgeYears, "birth_date", "age must be between 13 and 120 years")
	}
	v.Check(input.HeightCm >= minHeightCm && input.HeightCm <= maxHeightCm, "height_cm", "must be between 50 and 280")
	v.Check(input.WeightKg >= MinWeightKg && input.WeightKg <= MaxWeightKg, "weight_kg", "must be between 20 and 500")
	v.Check(IsValidActivityLevel(input.ActivityLevel), "activity_level",
		"must be sedentary, light, moderate, active or very_active")
	v.Check(input.Goal == GoalLose || input.Goal == GoalMaintain || input.Goal == GoalGain,
		"goal", "must be lose, maintain or gain")
	if goal := input.WeeklyWorkoutGoal; goal != nil {
		v.Check(*goal >= 0 && *goal <= maxWorkouts, "weekly_workout_goal", "must be between 0 and 14")
	}
	return v.Err()
}

func validateCredentials(email, password string) error {
	var v domain.Validator
	address, err := mail.ParseAddress(email)
	isPlainAddress := err == nil && address.Address == email && len(email) <= maxEmailSize
	v.Check(isPlainAddress, "email", "must be a valid email address")
	v.Check(len(password) >= minPasswordLength && len(password) <= maxPasswordLength,
		"password", "must be between 8 and 72 bytes long")
	return v.Err()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
