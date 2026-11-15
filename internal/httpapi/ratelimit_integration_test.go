package httpapi_test

import (
	"net/http"
	"testing"
)

func TestAuthEndpointsAreRateLimited(t *testing.T) {
	const limit = 3
	api := newTestAPIWithAuthLimit(t, limit)
	wrong := map[string]string{"email": "nobody@example.com", "password": "wrong-password"}

	for attempt := range limit {
		status, _ := api.call(http.MethodPost, "/api/v1/auth/login", "", wrong)
		if status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401 while within the limit", attempt+1, status)
		}
	}

	status, body := api.call(http.MethodPost, "/api/v1/auth/login", "", wrong)
	if status != http.StatusTooManyRequests || errorCode(body) != "rate_limited" {
		t.Fatalf("over the limit: got %d %q, want 429 rate_limited", status, errorCode(body))
	}
	// The test clock is frozen, so the whole 60-second window is still ahead.
	if got := api.lastHeader.Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After = %q, want 60", got)
	}

	// Register shares the same budget, so it cannot be used to keep guessing.
	credentials := map[string]string{"email": "new@example.com", "password": testPassword}
	if status, _ := api.call(http.MethodPost, "/api/v1/auth/register", "", credentials); status != http.StatusTooManyRequests {
		t.Errorf("register over the limit: status = %d, want 429", status)
	}

	// Everything else stays reachable.
	api.mustCall(http.StatusOK, http.MethodGet, "/healthz", "", nil)
}

func TestRateLimitDoesNotAffectAuthenticatedUse(t *testing.T) {
	const limit = 2
	api := newTestAPIWithAuthLimit(t, limit)
	token := api.registerWithProfile("busy@example.com") // one of the two auth calls

	for range limit * 5 {
		api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/profile", token, nil)
	}
}
