// Package auth hashes passwords and issues and verifies access tokens.
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidToken is returned for any token that cannot be trusted.
var ErrInvalidToken = errors.New("invalid token")

// PasswordHasher hashes passwords with bcrypt at a configurable cost.
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher returns a hasher using the given bcrypt cost.
func NewPasswordHasher(cost int) PasswordHasher {
	return PasswordHasher{cost: cost}
}

// Hash returns the bcrypt hash of password.
func (h PasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// IsMatch reports whether password corresponds to hash.
func (PasswordHasher) IsMatch(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// TokenManager issues and verifies HS256 JWT access tokens.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager returns a manager signing with secret; now is injected so
// expiry can be tested without sleeping.
func NewTokenManager(secret string, ttl time.Duration, now func() time.Time) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: now}
}

// Issue returns a signed token whose subject is userID.
func (m *TokenManager) Issue(userID int64) (string, error) {
	issuedAt := m.now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(issuedAt.Add(m.ttl)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return token, nil
}

// Parse verifies token and returns the user ID it was issued for.
func (m *TokenManager) Parse(token string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	// Pinning the method blocks algorithm-confusion attacks such as "none".
	_, err := jwt.ParseWithClaims(token, claims, m.key,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: subject is not a user id", ErrInvalidToken)
	}
	return userID, nil
}

func (m *TokenManager) key(*jwt.Token) (any, error) {
	return m.secret, nil
}
