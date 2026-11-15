// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultHTTPAddr     = ":8080"
	defaultDatabasePath = "fittrack.db"
	defaultJWTTTL       = 24 * time.Hour

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
