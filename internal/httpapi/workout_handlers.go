package httpapi

import (
	"net/http"

	"fittrack/internal/domain"
	"fittrack/internal/workout"
)

func (a *api) registerWorkoutRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+apiPrefix+"/activities", a.authenticated(a.listActivities))
	mux.Handle("GET "+apiPrefix+"/workouts", a.authenticated(a.listWorkouts))
	mux.Handle("POST "+apiPrefix+"/workouts", a.authenticated(a.createWorkout))
	mux.Handle("GET "+apiPrefix+"/workouts/{id}", a.authenticated(a.getWorkout))
	mux.Handle("PUT "+apiPrefix+"/workouts/{id}", a.authenticated(a.updateWorkout))
	mux.Handle("DELETE "+apiPrefix+"/workouts/{id}", a.authenticated(a.deleteWorkout))
}

func parseDateRange(r *http.Request) (domain.DateRange, error) {
	query := r.URL.Query()
	dates := domain.DateRange{From: query.Get("from"), To: query.Get("to")}
	var v domain.Validator
	v.Check(dates.From == "" || domain.IsDate(dates.From), "from", "must be a date in YYYY-MM-DD format")
	v.Check(dates.To == "" || domain.IsDate(dates.To), "to", "must be a date in YYYY-MM-DD format")
	return dates, v.Err()
}

func (a *api) listActivities(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, r, http.StatusOK, map[string][]workout.ActivityInfo{"data": workout.Activities()})
	return nil
}

func (a *api) listWorkouts(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePage(r)
	if err != nil {
		return err
	}
	dates, err := parseDateRange(r)
	if err != nil {
		return err
	}
	workouts, total, err := a.workouts.List(r.Context(), userIDFrom(r.Context()), dates, page)
	if err != nil {
		return err
	}
	writeList(w, r, workouts, page, total)
	return nil
}

func (a *api) createWorkout(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeJSON[workout.Input](w, r)
	if err != nil {
		return err
	}
	created, err := a.workouts.Create(r.Context(), userIDFrom(r.Context()), input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusCreated, created)
	return nil
}

func (a *api) getWorkout(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	found, err := a.workouts.Get(r.Context(), userIDFrom(r.Context()), id)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, found)
	return nil
}

func (a *api) updateWorkout(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	input, err := decodeJSON[workout.Input](w, r)
	if err != nil {
		return err
	}
	updated, err := a.workouts.Update(r.Context(), userIDFrom(r.Context()), id, input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, updated)
	return nil
}

func (a *api) deleteWorkout(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := a.workouts.Delete(r.Context(), userIDFrom(r.Context()), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
