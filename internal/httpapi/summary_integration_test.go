package httpapi_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return encoded
}

// logLunch logs 200 g rice and 100 g tempe (561 kcal, 26.8 g protein).
func logLunch(api *testAPI, token, date string) map[string]any {
	api.t.Helper()
	return api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/meals", token, map[string]any{
		"date": date, "meal_type": "lunch", "items": []map[string]any{
			{"food_id": api.foodID(token, "Nasi putih"), "grams": 200},
			{"food_id": api.foodID(token, "Tempe kedelai murni"), "grams": 100},
		},
	})
}

// logRun logs a 30-minute run (343 kcal at 70 kg).
func logRun(api *testAPI, token, date string) map[string]any {
	api.t.Helper()
	return api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/workouts", token,
		map[string]any{"activity": "running", "duration_min": 30, "date": date})
}

func TestDailySummary(t *testing.T) {
	api := newTestAPI(t)
	token := api.register("day@example.com")

	api.mustCall(http.StatusConflict, http.MethodGet, "/api/v1/summary/daily", token, nil) // no profile yet
	api.mustCall(http.StatusOK, http.MethodPut, "/api/v1/profile", token, referenceProfile())
	logLunch(api, token, today)
	logRun(api, token, today)
	logLunch(api, token, "2026-11-14") // another day must not leak into today

	// The date defaults to today. remaining = 2055.6 - 561 + 343.
	daily := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/daily", token, nil)
	if daily["date"] != today {
		t.Errorf("date = %v, want %s", daily["date"], today)
	}
	wantNumber(t, daily, "calories_in", 561)
	wantNumber(t, daily, "calories_burned", 343)
	wantNumber(t, daily, "target_kcal", 2055.6)
	wantNumber(t, daily, "remaining_kcal", 1837.6)
	wantNumber(t, daily, "protein_g", 26.8)
	wantNumber(t, daily, "meal_count", 1)
	wantNumber(t, daily, "workout_count", 1)

	quiet := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/daily?date=2026-11-01", token, nil)
	wantNumber(t, quiet, "calories_in", 0)
	wantNumber(t, quiet, "remaining_kcal", 2055.6)

	api.mustCall(http.StatusUnprocessableEntity, http.MethodGet, "/api/v1/summary/daily?date=yesterday", token, nil)
}

func TestWeeklySummaryAndCoach(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("week@example.com")
	logLunch(api, token, "2026-11-09")
	logLunch(api, token, "2026-11-10")
	logRun(api, token, "2026-11-10")
	logLunch(api, token, "2026-11-08") // the day before the window

	// week_start defaults to the Monday of the current week.
	weekly := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/weekly", token, nil)
	if weekly["week_start"] != weekStart || weekly["week_end"] != today {
		t.Errorf("window = %v..%v, want %s..%s", weekly["week_start"], weekly["week_end"], weekStart, today)
	}
	wantNumber(t, weekly, "workout_count", 1)
	wantNumber(t, weekly, "workout_goal", 3)
	wantNumber(t, weekly, "total_calories_in", 1122)
	wantNumber(t, weekly, "total_calories_burned", 343)
	days := weekly["days"].([]any)
	if len(days) != 7 {
		t.Fatalf("days = %d, want 7", len(days))
	}
	wantNumber(t, days[1].(map[string]any), "calories_burned", 343)

	advice := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/coach/weekly?week_start="+weekStart, token, nil)
	if advice["disclaimer"] == "" || advice["disclaimer"] == nil {
		t.Error("coach response has no disclaimer")
	}
	fired := make([]string, 0)
	for _, message := range advice["messages"].([]any) {
		fired = append(fired, message.(map[string]any)["rule"].(string))
	}
	// 5 trailing silent days, 1 of 3 workouts, 561 kcal (net) on 2 days only,
	// and 26.8 g protein against a 56 g minimum.
	want := []string{"no_recent_logs", "workouts_below_goal", "low_protein"}
	if !slices.Equal(fired, want) {
		t.Errorf("coach fired %v, want %v", fired, want)
	}

	future := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/weekly?week_start=2026-11-16", token, nil)
	if future["days"].([]any)[0].(map[string]any)["is_elapsed"] != false {
		t.Error("a future day is marked as elapsed")
	}
}

func TestAuthorizationBoundary(t *testing.T) {
	api := newTestAPI(t)
	owner := api.registerWithProfile("owner@example.com")
	intruder := api.registerWithProfile("intruder@example.com")

	workout := logRun(api, owner, today)
	meal := logLunch(api, owner, today)
	custom := api.mustCall(http.StatusCreated, http.MethodPost, "/api/v1/foods", owner, map[string]any{
		"name": "Resep rahasia", "kcal_per_100g": 100, "protein_g_per_100g": 5, "carbs_g_per_100g": 10, "fat_g_per_100g": 3,
	})

	run := map[string]any{"activity": "walking", "duration_min": 10, "date": today}
	lunch := map[string]any{"date": today, "meal_type": "lunch",
		"items": []map[string]any{{"food_id": api.foodID(intruder, "Tahu"), "grams": 100}}}
	foodEdit := map[string]any{"name": "Dibajak", "kcal_per_100g": 1,
		"protein_g_per_100g": 0, "carbs_g_per_100g": 0, "fat_g_per_100g": 0}

	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"read workout", http.MethodGet, itemPath("workouts", id(workout)), nil},
		{"update workout", http.MethodPut, itemPath("workouts", id(workout)), run},
		{"delete workout", http.MethodDelete, itemPath("workouts", id(workout)), nil},
		{"read meal", http.MethodGet, itemPath("meals", id(meal)), nil},
		{"update meal", http.MethodPut, itemPath("meals", id(meal)), lunch},
		{"delete meal", http.MethodDelete, itemPath("meals", id(meal)), nil},
		{"read custom food", http.MethodGet, itemPath("foods", id(custom)), nil},
		{"update custom food", http.MethodPut, itemPath("foods", id(custom)), foodEdit},
		{"delete custom food", http.MethodDelete, itemPath("foods", id(custom)), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 404, not 403: the intruder must not learn the resource exists.
			if status, body := api.call(tt.method, tt.path, intruder, tt.body); status != http.StatusNotFound {
				t.Errorf("status = %d, want 404; body = %v", status, body)
			}
		})
	}

	assertIntruderSeesNothing(t, api, intruder, id(custom))
	assertOwnerDataIntact(t, api, owner, id(workout), id(meal), id(custom))
}

func assertIntruderSeesNothing(t *testing.T, api *testAPI, intruder string, customFoodID int64) {
	t.Helper()
	for _, path := range []string{"/api/v1/workouts", "/api/v1/meals", "/api/v1/foods?q=rahasia"} {
		listed := api.mustCall(http.StatusOK, http.MethodGet, path, intruder, nil)
		wantNumber(t, listed["pagination"].(map[string]any), "total", 0)
	}
	daily := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/daily", intruder, nil)
	wantNumber(t, daily, "calories_in", 0)
	wantNumber(t, daily, "calories_burned", 0)

	// Another user's custom food cannot be referenced in a meal either.
	api.mustCall(http.StatusUnprocessableEntity, http.MethodPost, "/api/v1/meals", intruder, map[string]any{
		"date": today, "meal_type": "snack", "items": []map[string]any{{"food_id": customFoodID, "grams": 50}},
	})
}

func assertOwnerDataIntact(t *testing.T, api *testAPI, owner string, workoutID, mealID, foodID int64) {
	t.Helper()
	workout := api.mustCall(http.StatusOK, http.MethodGet, itemPath("workouts", workoutID), owner, nil)
	if workout["activity"] != "running" {
		t.Errorf("owner's workout was modified: %v", workout)
	}
	meal := api.mustCall(http.StatusOK, http.MethodGet, itemPath("meals", mealID), owner, nil)
	wantNumber(t, meal["totals"].(map[string]any), "kcal", 561)
	food := api.mustCall(http.StatusOK, http.MethodGet, itemPath("foods", foodID), owner, nil)
	if food["name"] != "Resep rahasia" {
		t.Errorf("owner's food was modified: %v", food)
	}
}
