package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAuthFlow(t *testing.T) {
	api := newTestAPI(t)
	credentials := map[string]string{"email": "Budi@Example.com", "password": testPassword}

	created := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/auth/register", "", credentials)
	if email := created["user"].(map[string]any)["email"]; email != "budi@example.com" {
		t.Errorf("registered email = %v, want it lowercased", email)
	}
	if strings.Contains(strings.ToLower(string(mustJSON(t, created))), "password") {
		t.Error("register response leaks password data")
	}

	duplicate := api.mustCall(http.StatusConflict, http.MethodPost, "/api/v1/auth/register", "", credentials)
	if errorCode(duplicate) != "conflict" {
		t.Errorf("duplicate register code = %q, want conflict", errorCode(duplicate))
	}

	login := api.mustCall(http.StatusOK, http.MethodPost, "/api/v1/auth/login", "", credentials)
	token := login["token"].(string)
	api.mustCall(http.StatusConflict, http.MethodGet, "/api/v1/profile", token, nil) // profile_required, but authenticated

	wrong := map[string]string{"email": "budi@example.com", "password": "wrong-password"}
	api.mustCall(http.StatusUnauthorized, http.MethodPost, "/api/v1/auth/login", "", wrong)
	unknown := map[string]string{"email": "nobody@example.com", "password": testPassword}
	api.mustCall(http.StatusUnauthorized, http.MethodPost, "/api/v1/auth/login", "", unknown)
}

func TestAuthRejectsBadInputAndTokens(t *testing.T) {
	api := newTestAPI(t)
	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		body       any
		wantStatus int
		wantCode   string
	}{
		{"invalid email", http.MethodPost, "/api/v1/auth/register", "",
			map[string]string{"email": "not-an-email", "password": testPassword}, http.StatusUnprocessableEntity, "validation_failed"},
		{"short password", http.MethodPost, "/api/v1/auth/register", "",
			map[string]string{"email": "a@example.com", "password": "short"}, http.StatusUnprocessableEntity, "validation_failed"},
		{"malformed json", http.MethodPost, "/api/v1/auth/register", "", `{"email":`, http.StatusBadRequest, "bad_request"},
		{"unknown field", http.MethodPost, "/api/v1/auth/register", "",
			`{"email":"a@example.com","password":"long-enough-pw","role":"admin"}`, http.StatusBadRequest, "bad_request"},
		{"missing token", http.MethodGet, "/api/v1/profile", "", nil, http.StatusUnauthorized, "unauthorized"},
		{"garbage token", http.MethodGet, "/api/v1/workouts", "garbage", nil, http.StatusUnauthorized, "unauthorized"},
		{"unknown route", http.MethodGet, "/api/v1/nope", "", nil, http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := api.call(tt.method, tt.path, tt.token, tt.body)
			if status != tt.wantStatus || errorCode(body) != tt.wantCode {
				t.Errorf("got %d %q, want %d %q; body = %v", status, errorCode(body), tt.wantStatus, tt.wantCode, body)
			}
		})
	}
}

func TestProfile(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("siti@example.com")

	missing := api.mustCall(http.StatusConflict, http.MethodGet, "/api/v1/profile", token, nil)
	if errorCode(missing) != "profile_required" {
		t.Errorf("code = %q, want profile_required", errorCode(missing))
	}

	saved := api.mustCall(http.StatusOK, http.MethodPut, "/api/v1/profile", token, referenceProfile())
	wantNumber(t, saved, "bmr_kcal", 1648.8)
	wantNumber(t, saved, "tdee_kcal", 2555.6)
	wantNumber(t, saved, "daily_target_kcal", 2055.6)
	wantNumber(t, saved, "weekly_workout_goal", 3)

	fetched := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/profile", token, nil)
	wantNumber(t, fetched, "daily_target_kcal", 2055.6)

	invalid := referenceProfile()
	invalid["height_cm"] = 1.75 // metres instead of centimetres
	invalid["goal"] = "shred"
	rejected := api.mustCall(http.StatusUnprocessableEntity, http.MethodPut, "/api/v1/profile", token, invalid)
	details := rejected["error"].(map[string]any)["details"].([]any)
	if len(details) != 2 {
		t.Errorf("validation details = %v, want one entry per invalid field", details)
	}
}

func TestFoods(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("food@example.com")

	page := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/foods?limit=5", token, nil)
	if got := len(page["data"].([]any)); got != 5 {
		t.Errorf("page size = %d, want 5", got)
	}
	wantNumber(t, page["pagination"].(map[string]any), "total", 13)

	search := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/foods?q=tempe", token, nil)
	wantNumber(t, search["pagination"].(map[string]any), "total", 1)
	api.mustCall(http.StatusUnprocessableEntity, http.MethodGet, "/api/v1/foods?limit=0", token, nil)

	custom := map[string]any{"name": "Nasi goreng rumah", "kcal_per_100g": 170,
		"protein_g_per_100g": 4, "carbs_g_per_100g": 25, "fat_g_per_100g": 6}
	created := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/foods", token, custom)
	if created["is_custom"] != true || created["verification_status"] != "user_provided" {
		t.Errorf("custom food = %v, want is_custom and user_provided", created)
	}

	custom["kcal_per_100g"] = 180
	updated := api.mustCall(http.StatusOK, http.MethodPut, itemPath("foods", id(created)), token, custom)
	wantNumber(t, updated, "kcal_per_100g", 180)

	custom["fat_g_per_100g"] = 90 // macros would exceed 100 g per 100 g
	api.mustCall(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/foods", token, custom)
	custom["fat_g_per_100g"] = 6

	seeded := itemPath("foods", api.foodID(token, "Nasi putih"))
	api.mustCall(http.StatusForbidden, http.MethodPut, seeded, token, custom)
	api.mustCall(http.StatusForbidden, http.MethodDelete, seeded, token, nil)

	api.mustCall(http.StatusNoContent, http.MethodDelete, itemPath("foods", id(created)), token, nil)
	api.mustCall(http.StatusNotFound, http.MethodGet, itemPath("foods", id(created)), token, nil)
}

func TestWorkoutFlow(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("run@example.com")
	run := map[string]any{"activity": "running", "duration_min": 30, "distance_km": 5, "date": today}

	noProfile := api.mustCall(http.StatusConflict, http.MethodPost, "/api/v1/workouts", token, run)
	if errorCode(noProfile) != "profile_required" {
		t.Errorf("code = %q, want profile_required", errorCode(noProfile))
	}
	api.mustCall(http.StatusOK, http.MethodPut, "/api/v1/profile", token, referenceProfile())

	// MET 9.8 x 70 kg x 0.5 h = 343 kcal.
	created := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/workouts", token, run)
	wantNumber(t, created, "calories_burned", 343)
	path := itemPath("workouts", id(created))

	fetched := api.mustCall(http.StatusOK, http.MethodGet, path, token, nil)
	wantNumber(t, fetched, "distance_km", 5)

	run["duration_min"] = 60
	updated := api.mustCall(http.StatusOK, http.MethodPut, path, token, run)
	wantNumber(t, updated, "calories_burned", 686)

	listed := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/workouts?from="+today+"&to="+today, token, nil)
	wantNumber(t, listed["pagination"].(map[string]any), "total", 1)
	outside := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/workouts?to=2026-01-01", token, nil)
	wantNumber(t, outside["pagination"].(map[string]any), "total", 0)

	invalid := map[string]any{"activity": "teleporting", "duration_min": -5, "date": "15-11-2026"}
	api.mustCall(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/workouts", token, invalid)

	api.mustCall(http.StatusNoContent, http.MethodDelete, path, token, nil)
	api.mustCall(http.StatusNotFound, http.MethodGet, path, token, nil)
	api.mustCall(http.StatusNotFound, http.MethodDelete, path, token, nil)

	activities := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/activities", token, nil)
	if got := len(activities["data"].([]any)); got != 8 {
		t.Errorf("activities = %d, want 8", got)
	}
}

func TestMealFlow(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("eat@example.com")
	rice, tempe := api.foodID(token, "Nasi putih"), api.foodID(token, "Tempe kedelai murni")
	lunch := map[string]any{"date": today, "meal_type": "lunch", "items": []map[string]any{
		{"food_id": rice, "grams": 200}, {"food_id": tempe, "grams": 100},
	}}

	// Rice 200 g: 360 kcal, 6 P, 79.6 C, 0.6 F. Tempe 100 g: 201 kcal, 20.8 P, 13.5 C, 8.8 F.
	created := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/meals", token, lunch)
	totals := created["totals"].(map[string]any)
	wantNumber(t, totals, "kcal", 561)
	wantNumber(t, totals, "protein_g", 26.8)
	wantNumber(t, totals, "carbs_g", 93.1)
	wantNumber(t, totals, "fat_g", 9.4)
	if got := len(created["items"].([]any)); got != 2 {
		t.Fatalf("items = %d, want 2", got)
	}
	path := itemPath("meals", id(created))

	lunch["items"] = []map[string]any{{"food_id": rice, "grams": 100}}
	updated := api.mustCall(http.StatusOK, http.MethodPut, path, token, lunch)
	wantNumber(t, updated["totals"].(map[string]any), "kcal", 180)
	if got := len(updated["items"].([]any)); got != 1 {
		t.Errorf("items after update = %d, want 1", got)
	}

	listed := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/meals?from="+today, token, nil)
	first := listed["data"].([]any)[0].(map[string]any)
	wantNumber(t, first["totals"].(map[string]any), "kcal", 180)

	invalid := map[string]any{"date": today, "meal_type": "brunch", "items": []map[string]any{
		{"food_id": 999999, "grams": 0},
	}}
	rejected := api.mustCall(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/meals", token, invalid)
	if got := len(rejected["error"].(map[string]any)["details"].([]any)); got != 3 {
		t.Errorf("validation details = %d, want 3 (meal_type, grams, food_id)", got)
	}
	empty := map[string]any{"date": today, "meal_type": "snack", "items": []any{}}
	api.mustCall(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/meals", token, empty)

	api.mustCall(http.StatusNoContent, http.MethodDelete, path, token, nil)
	api.mustCall(http.StatusNotFound, http.MethodGet, path, token, nil)
}

func TestFoodInUseCannotBeDeleted(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("keep@example.com")
	custom := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/foods", token, map[string]any{
		"name": "Bubur ayam", "kcal_per_100g": 100, "protein_g_per_100g": 5, "carbs_g_per_100g": 15, "fat_g_per_100g": 2,
	})
	meal := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/meals", token, map[string]any{
		"date": today, "meal_type": "breakfast", "items": []map[string]any{{"food_id": id(custom), "grams": 250}},
	})

	api.mustCall(http.StatusConflict, http.MethodDelete, itemPath("foods", id(custom)), token, nil)
	api.mustCall(http.StatusNoContent, http.MethodDelete, itemPath("meals", id(meal)), token, nil)
	api.mustCall(http.StatusNoContent, http.MethodDelete, itemPath("foods", id(custom)), token, nil)
}
