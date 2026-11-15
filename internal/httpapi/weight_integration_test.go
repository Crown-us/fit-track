package httpapi_test

import (
	"net/http"
	"testing"
)

const (
	weightsPath = "/api/v1/weights"
	earlierDay  = "2026-11-10"
)

func weightPath(date string) string {
	return weightsPath + "/" + date
}

func TestWeightHistory(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("weight@example.com")

	// Saving the profile logged its 70 kg as today's entry.
	initial := api.mustCall(http.StatusOK, http.MethodGet, weightsPath, token, nil)
	wantNumber(t, initial["pagination"].(map[string]any), "total", 1)
	first := initial["data"].([]any)[0].(map[string]any)
	if first["date"] != today {
		t.Errorf("first entry date = %v, want %s", first["date"], today)
	}
	wantNumber(t, first, "weight_kg", 70)

	// An older entry is history only: the current weight stays 70 kg.
	saved := api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), token, map[string]any{"weight_kg": 72})
	wantNumber(t, saved, "weight_kg", 72)
	profile := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/profile", token, nil)
	wantNumber(t, profile, "weight_kg", 70)

	listed := api.mustCall(http.StatusOK, http.MethodGet, weightsPath+"?from="+earlierDay+"&to="+earlierDay, token, nil)
	wantNumber(t, listed["pagination"].(map[string]any), "total", 1)

	// Logging today again replaces the entry and becomes the current weight.
	// BMR 1628.75, TDEE 2524.5625, target 2024.5625.
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(today), token, map[string]any{"weight_kg": 68})
	profile = api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/profile", token, nil)
	wantNumber(t, profile, "weight_kg", 68)
	wantNumber(t, profile, "daily_target_kcal", 2024.6)
	all := api.mustCall(http.StatusOK, http.MethodGet, weightsPath, token, nil)
	wantNumber(t, all["pagination"].(map[string]any), "total", 2)
}

func TestWeightValidation(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("valid@example.com")
	tests := []struct {
		name string
		date string
		body any
	}{
		{"future date", "2026-11-16", map[string]any{"weight_kg": 70}},
		{"not a date", "kemarin", map[string]any{"weight_kg": 70}},
		{"too light", today, map[string]any{"weight_kg": 5}},
		{"too heavy", today, map[string]any{"weight_kg": 900}},
		{"missing weight", today, map[string]any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := api.call(http.MethodPut, weightPath(tt.date), token, tt.body)
			if status != http.StatusUnprocessableEntity || errorCode(body) != "validation_failed" {
				t.Errorf("got %d %q, want 422 validation_failed", status, errorCode(body))
			}
		})
	}
}

func TestWeightDelete(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("delete@example.com")
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), token, map[string]any{"weight_kg": 72})

	api.mustCall(http.StatusNoContent, http.MethodDelete, weightPath(earlierDay), token, nil)
	api.mustCall(http.StatusNotFound, http.MethodDelete, weightPath(earlierDay), token, nil)

	// The only remaining entry is protected, and stays in place.
	api.mustCall(http.StatusUnprocessableEntity, http.MethodDelete, weightPath(today), token, nil)
	remaining := api.mustCall(http.StatusOK, http.MethodGet, weightsPath, token, nil)
	wantNumber(t, remaining["pagination"].(map[string]any), "total", 1)
}

func TestDeletingLatestWeightRestoresPreviousOne(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("restore@example.com")
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), token, map[string]any{"weight_kg": 72})

	api.mustCall(http.StatusNoContent, http.MethodDelete, weightPath(today), token, nil)
	profile := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/profile", token, nil)
	wantNumber(t, profile, "weight_kg", 72)
}

func TestWorkoutsAndSummariesUseWeightOfTheDay(t *testing.T) {
	api := newTestAPI(t)
	token := api.registerWithProfile("history@example.com") // 70 kg today
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), token, map[string]any{"weight_kg": 72})

	tests := []struct {
		name string
		date string
		want float64 // MET 9.8 x weight x 0.5 h
	}{
		{"on the day of the older entry", earlierDay, 352.8},
		{"between entries uses the latest earlier one", "2026-11-12", 352.8},
		{"before all entries uses the earliest one", "2026-11-01", 352.8},
		{"today uses today's entry", today, 343},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantNumber(t, logRun(api, token, tt.date), "calories_burned", tt.want)
		})
	}

	// At 72 kg: BMR 1668.75, TDEE 2586.5625, target 2086.5625.
	past := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/daily?date="+earlierDay, token, nil)
	wantNumber(t, past, "weight_kg", 72)
	wantNumber(t, past, "target_kcal", 2086.6)
	current := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/daily", token, nil)
	wantNumber(t, current, "weight_kg", 70)
	wantNumber(t, current, "target_kcal", 2055.6)

	weekly := api.mustCall(http.StatusOK, http.MethodGet, "/api/v1/summary/weekly?week_start="+weekStart, token, nil)
	wantNumber(t, weekly, "weight_kg", 70) // the most recent elapsed day
	wantNumber(t, weekly, "target_kcal", 2055.6)
	days := weekly["days"].([]any)
	wantNumber(t, days[0].(map[string]any), "target_kcal", 2086.6) // Monday the 9th falls back to the 72 kg entry
	wantNumber(t, days[6].(map[string]any), "target_kcal", 2055.6)
}

func TestWeightAuthorizationBoundary(t *testing.T) {
	api := newTestAPI(t)
	owner := api.registerWithProfile("w-owner@example.com")
	intruder := api.registerWithProfile("w-intruder@example.com")
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), owner, map[string]any{"weight_kg": 72})

	// The intruder sees only their own profile entry and cannot delete the owner's.
	theirs := api.mustCall(http.StatusOK, http.MethodGet, weightsPath, intruder, nil)
	wantNumber(t, theirs["pagination"].(map[string]any), "total", 1)
	api.mustCall(http.StatusNotFound, http.MethodDelete, weightPath(earlierDay), intruder, nil)

	// Writing the same date creates the intruder's own entry, not an overwrite.
	api.mustCall(http.StatusOK, http.MethodPut, weightPath(earlierDay), intruder, map[string]any{"weight_kg": 90})
	owners := api.mustCall(http.StatusOK, http.MethodGet, weightsPath+"?to="+earlierDay, owner, nil)
	wantNumber(t, owners["data"].([]any)[0].(map[string]any), "weight_kg", 72)

	api.mustCall(http.StatusUnauthorized, http.MethodGet, weightsPath, "", nil)
}
