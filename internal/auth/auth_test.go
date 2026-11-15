package auth

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret-test-secret-test-secret"

func TestPasswordHasher(t *testing.T) {
	hasher := NewPasswordHasher(bcrypt.MinCost)
	hash, err := hasher.Hash("correct horse")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if !hasher.IsMatch(hash, "correct horse") {
		t.Error("IsMatch() = false for the right password")
	}
	if hasher.IsMatch(hash, "wrong horse") {
		t.Error("IsMatch() = true for the wrong password")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	manager := NewTokenManager(testSecret, time.Hour, time.Now)
	token, err := manager.Issue(42)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	userID, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if userID != 42 {
		t.Errorf("Parse() = %d, want 42", userID)
	}
}

func TestTokenRejected(t *testing.T) {
	issuedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return issuedAt }
	manager := NewTokenManager(testSecret, time.Hour, clock)
	token, err := manager.Issue(7)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	later := func() time.Time { return issuedAt.Add(2 * time.Hour) }
	tests := []struct {
		name    string
		manager *TokenManager
		token   string
	}{
		{name: "expired", manager: NewTokenManager(testSecret, time.Hour, later), token: token},
		{name: "wrong secret", manager: NewTokenManager(testSecret+"x", time.Hour, clock), token: token},
		{name: "tampered", manager: manager, token: token + "x"},
		{name: "garbage", manager: manager, token: "not-a-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.manager.Parse(tt.token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Parse() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}
