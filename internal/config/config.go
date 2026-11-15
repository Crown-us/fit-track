// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	_ "time/tzdata" // Embedded so APP_TIMEZONE works on hosts and images without a timezone database.

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultHTTPAddr     = ":8080"
	defaultDatabasePath = "fittrack.db"
	defaultJWTTTL       = 24 * time.Hour
	// defaultTimezone decides when a calendar day starts. The product targets
	// Indonesian users, so the default is Western Indonesia Time.
	defaultTimezone = "Asia/Jakarta"

	// minJWTSecretBytes matches the HS256 key size (256 bits) so the secret
	// is never weaker than the signature algorithm.
	minJWTSecretBytes = 32
)

// Config holds every setting the API needs at startup.
type Config struct {
	HTTPAddr     string
	DatabasePath string
	JWTSecret    string
	JWTTTL       time.Duration
	BcryptCost   int
	LogLevel     slog.Level
	// Location is the timezone in which "today" and week boundaries are
	// evaluated.
	Location *time.Location
}

// Load reads configuration through getenv (os.Getenv in production) so tests
// can supply values without touching the process environment.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:     valueOr(getenv("HTTP_ADDR"), defaultHTTPAddr),
		DatabasePath: valueOr(getenv("DATABASE_PATH"), defaultDatabasePath),
		JWTSecret:    getenv("JWT_SECRET"),
		JWTTTL:       defaultJWTTTL,
		BcryptCost:   bcrypt.DefaultCost,
		LogLevel:     slog.LevelInfo,
	}
	if len(cfg.JWTSecret) < minJWTSecretBytes {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least %d bytes", minJWTSecretBytes)
	}
	if err := loadOptional(getenv, &cfg); err != nil {
		return Config{}, err
	}
	location, err := time.LoadLocation(valueOr(getenv("APP_TIMEZONE"), defaultTimezone))
	if err != nil {
		return Config{}, fmt.Errorf("APP_TIMEZONE: %w", err)
	}
	cfg.Location = location
	return cfg, nil
}

func loadOptional(getenv func(string) string, cfg *Config) error {
	if raw := getenv("JWT_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return errors.New("JWT_TTL must be a positive duration such as 24h")
		}
		cfg.JWTTTL = ttl
	}
	if raw := getenv("BCRYPT_COST"); raw != "" {
		cost, err := strconv.Atoi(raw)
		if err != nil || cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
			return fmt.Errorf("BCRYPT_COST must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
		}
		cfg.BcryptCost = cost
	}
	if raw := getenv("LOG_LEVEL"); raw != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(raw)); err != nil {
			return fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	return nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
