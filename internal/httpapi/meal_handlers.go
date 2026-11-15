package httpapi

import (
	"net/http"

	"github.com/Crown-us/fit-track/internal/meal"
)

func (a *api) registerMealRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+apiPrefix+"/meals", a.authenticated(a.listMeals))
	mux.Handle("POST "+apiPrefix+"/meals", a.authenticated(a.createMeal))
	mux.Handle("GET "+apiPrefix+"/meals/{id}", a.authenticated(a.getMeal))
	mux.Handle("PUT "+apiPrefix+"/meals/{id}", a.authenticated(a.updateMeal))
	mux.Handle("DELETE "+apiPrefix+"/meals/{id}", a.authenticated(a.deleteMeal))
}

func (a *api) listMeals(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePage(r)
	if err != nil {
		return err
	}
	dates, err := parseDateRange(r)
	if err != nil {
		return err
	}
	meals, total, err := a.meals.List(r.Context(), userIDFrom(r.Context()), dates, page)
	if err != nil {
		return err
	}
	writeList(w, r, meals, page, total)
	return nil
}

func (a *api) createMeal(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeJSON[meal.Input](w, r)
	if err != nil {
		return err
	}
	created, err := a.meals.Create(r.Context(), userIDFrom(r.Context()), input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusCreated, created)
	return nil
}

func (a *api) getMeal(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	found, err := a.meals.Get(r.Context(), userIDFrom(r.Context()), id)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, found)
	return nil
}

func (a *api) updateMeal(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	input, err := decodeJSON[meal.Input](w, r)
	if err != nil {
		return err
	}
	updated, err := a.meals.Update(r.Context(), userIDFrom(r.Context()), id, input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, updated)
	return nil
}

func (a *api) deleteMeal(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := a.meals.Delete(r.Context(), userIDFrom(r.Context()), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
