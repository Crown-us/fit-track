package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad(t *testing.T) {
	secret := strings.Repeat("s", minJWTSecretBytes)
	tests := []struct {
		name      string
		values    map[string]string
		wantError bool
	}{
		{name: "defaults", values: map[string]string{"JWT_SECRET": secret}},
		{name: "missing secret", values: map[string]string{}, wantError: true},
		{name: "short secret", values: map[string]string{"JWT_SECRET": "short"}, wantError: true},
		{name: "bad ttl", values: map[string]string{"JWT_SECRET": secret, "JWT_TTL": "soon"}, wantError: true},
		{name: "bad cost", values: map[string]string{"JWT_SECRET": secret, "BCRYPT_COST": "99"}, wantError: true},
		{name: "bad level", values: map[string]string{"JWT_SECRET": secret, "LOG_LEVEL": "loud"}, wantError: true},
		{name: "zero rate limit", values: map[string]string{"JWT_SECRET": secret, "AUTH_RATE_LIMIT": "0"}, wantError: true},
		{name: "bad rate window", values: map[string]string{"JWT_SECRET": secret, "AUTH_RATE_WINDOW": "-1m"}, wantError: true},
		{name: "bad proxy flag", values: map[string]string{"JWT_SECRET": secret, "TRUST_PROXY_HEADERS": "maybe"}, wantError: true},
		{name: "bad timezone", values: map[string]string{"JWT_SECRET": secret, "APP_TIMEZONE": "Mars/Olympus"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(env(tt.values))
			if (err != nil) != tt.wantError {
				t.Fatalf("Load() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"JWT_SECRET":   strings.Repeat("s", minJWTSecretBytes),
		"HTTP_ADDR":    ":9000",
		"JWT_TTL":      "2h",
		"LOG_LEVEL":    "debug",
		"APP_TIMEZONE": "Asia/Makassar",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":9000" || cfg.JWTTTL != 2*time.Hour || cfg.LogLevel != slog.LevelDebug ||
		cfg.Location.String() != "Asia/Makassar" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}
