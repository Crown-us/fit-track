package httpapi

import (
	"net/http"
	"time"

	"github.com/Crown-us/fit-track/internal/domain"
)

func (a *api) registerSummaryRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+apiPrefix+"/summary/daily", a.authenticated(a.dailySummary))
	mux.Handle("GET "+apiPrefix+"/summary/weekly", a.authenticated(a.weeklySummary))
	mux.Handle("GET "+apiPrefix+"/coach/weekly", a.authenticated(a.weeklyCoach))
}

// dateParam reads an optional YYYY-MM-DD query parameter, falling back to
// fallback when the client omits it.
func dateParam(r *http.Request, name string, fallback time.Time) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	date, err := domain.ParseDate(raw)
	if err != nil {
		var v domain.Validator
		v.Check(false, name, "must be a date in YYYY-MM-DD format")
		return time.Time{}, v.Err()
	}
	return date, nil
}

func (a *api) dailySummary(w http.ResponseWriter, r *http.Request) error {
	date, err := dateParam(r, "date", a.summaries.Today())
	if err != nil {
		return err
	}
	daily, err := a.summaries.Daily(r.Context(), userIDFrom(r.Context()), date)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, daily)
	return nil
}

func (a *api) weeklySummary(w http.ResponseWriter, r *http.Request) error {
	weekStart, err := dateParam(r, "week_start", a.summaries.CurrentWeekStart())
	if err != nil {
		return err
	}
	weekly, err := a.summaries.Weekly(r.Context(), userIDFrom(r.Context()), weekStart)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, weekly)
	return nil
}

func (a *api) weeklyCoach(w http.ResponseWriter, r *http.Request) error {
	weekStart, err := dateParam(r, "week_start", a.summaries.CurrentWeekStart())
	if err != nil {
		return err
	}
	weekly, err := a.summaries.Weekly(r.Context(), userIDFrom(r.Context()), weekStart)
	if err != nil {
		return err
	}
	advice, err := a.coach.Advise(r.Context(), weekly)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, advice)
	return nil
}
