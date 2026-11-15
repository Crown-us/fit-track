package httpapi

import (
	"net/http"

	"github.com/Crown-us/fit-track/internal/weight"
)

type weightRequest struct {
	WeightKg float64 `json:"weight_kg"`
}

func (a *api) registerWeightRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+apiPrefix+"/weights", a.authenticated(a.listWeights))
	mux.Handle("PUT "+apiPrefix+"/weights/{date}", a.authenticated(a.logWeight))
	mux.Handle("DELETE "+apiPrefix+"/weights/{date}", a.authenticated(a.deleteWeight))
}

func (a *api) listWeights(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePage(r)
	if err != nil {
		return err
	}
	dates, err := parseDateRange(r)
	if err != nil {
		return err
	}
	entries, total, err := a.weights.List(r.Context(), userIDFrom(r.Context()), dates, page)
	if err != nil {
		return err
	}
	writeList(w, r, entries, page, total)
	return nil
}

// logWeight is a PUT keyed by date because a day has exactly one weight:
// repeating the request is harmless and replaces the value.
func (a *api) logWeight(w http.ResponseWriter, r *http.Request) error {
	request, err := decodeJSON[weightRequest](w, r)
	if err != nil {
		return err
	}
	entry := weight.Entry{Date: r.PathValue("date"), WeightKg: request.WeightKg}
	saved, err := a.weights.Log(r.Context(), userIDFrom(r.Context()), entry)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, saved)
	return nil
}

func (a *api) deleteWeight(w http.ResponseWriter, r *http.Request) error {
	if err := a.weights.Delete(r.Context(), userIDFrom(r.Context()), r.PathValue("date")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
