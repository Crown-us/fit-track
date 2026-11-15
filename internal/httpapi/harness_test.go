package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"fittrack/internal/auth"
	"fittrack/internal/food"
	"fittrack/internal/httpapi"
	"fittrack/internal/store"
)

const (
	testSecret   = "integration-test-secret-0123456789abcdef"
	testPassword = "correct-horse-battery"
	// today is a Sunday; its week starts on Monday 2026-11-09.
	today     = "2026-11-15"
	weekStart = "2026-11-09"
)

// fixedNow pins "today" so age, elapsed days and defaults are deterministic.
func fixedNow() time.Time {
	return time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
}

// testAPI is a running API backed by its own temporary SQLite database.
type testAPI struct {
	t       *testing.T
	baseURL string
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := food.Seed(ctx, db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	server := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:     db,
		Tokens: auth.NewTokenManager(testSecret, time.Hour, fixedNow),
		Hasher: auth.NewPasswordHasher(bcrypt.MinCost),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    fixedNow,
	}))
	t.Cleanup(func() {
		server.Close()
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return &testAPI{t: t, baseURL: server.URL}
}

// call sends a request and returns the status and decoded JSON object.
func (a *testAPI) call(method, path, token string, body any) (int, map[string]any) {
	a.t.Helper()
	status, raw := a.callRaw(method, path, token, body)
	if len(raw) == 0 {
		return status, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		a.t.Fatalf("%s %s: response is not a JSON object: %s", method, path, raw)
	}
	return status, decoded
}

func (a *testAPI) callRaw(method, path, token string, body any) (int, []byte) {
	a.t.Helper()
	var payload io.Reader
	switch typed := body.(type) {
	case nil:
	case string:
		payload = bytes.NewBufferString(typed)
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("encode body: %v", err)
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, a.baseURL+path, payload)
	if err != nil {
		a.t.Fatalf("build request: %v", err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close() //nolint:errcheck // Nothing to do about a failed close in a test.
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		a.t.Fatalf("read response: %v", err)
	}
	return response.StatusCode, raw
}

// mustCall fails the test unless the response has the wanted status.
func (a *testAPI) mustCall(wantStatus int, method, path, token string, body any) map[string]any {
	a.t.Helper()
	status, decoded := a.call(method, path, token, body)
	if status != wantStatus {
		a.t.Fatalf("%s %s: status = %d, want %d; body = %v", method, path, status, wantStatus, decoded)
	}
	return decoded
}

// register creates an account and returns its token.
func (a *testAPI) register(email string) string {
	a.t.Helper()
	body := a.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", "",
		map[string]string{"email": email, "password": testPassword})
	return body["token"].(string)
}

// registerWithProfile creates an account with the reference profile:
// male, 70 kg, 175 cm, 30 years old on the fixed date, moderate, lose.
// BMR 1648.75, TDEE 2555.5625, target 2055.5625 (rounded 2055.6).
func (a *testAPI) registerWithProfile(email string) string {
	a.t.Helper()
	token := a.register(email)
	a.mustCall(http.StatusOK, http.MethodPut, "/api/v1/profile", token, referenceProfile())
	return token
}

func referenceProfile() map[string]any {
	return map[string]any{
		"sex": "male", "birth_date": "1996-06-15", "height_cm": 175, "weight_kg": 70,
		"activity_level": "moderate", "goal": "lose",
	}
}

// foodID looks up a seeded food by its exact name.
func (a *testAPI) foodID(token, name string) int64 {
	a.t.Helper()
	body := a.mustCall(http.StatusOK, http.MethodGet, "/api/v1/foods?limit=100", token, nil)
	for _, item := range body["data"].([]any) {
		entry := item.(map[string]any)
		if entry["name"] == name {
			return int64(entry["id"].(float64))
		}
	}
	a.t.Fatalf("food %q not found", name)
	return 0
}

func id(body map[string]any) int64 {
	return int64(body["id"].(float64))
}

func errorCode(body map[string]any) string {
	detail, _ := body["error"].(map[string]any)
	code, _ := detail["code"].(string)
	return code
}

func wantNumber(t *testing.T, body map[string]any, key string, want float64) {
	t.Helper()
	got, ok := body[key].(float64)
	if !ok || got != want {
		t.Errorf("%s = %v, want %v", key, body[key], want)
	}
}

func itemPath(resource string, itemID int64) string {
	return fmt.Sprintf("/api/v1/%s/%d", resource, itemID)
}
