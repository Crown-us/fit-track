package httpapi

import (
	"net/http"

	"fittrack/internal/food"
)

func (a *api) registerFoodRoutes(mux *http.ServeMux) {
	mux.Handle("GET "+apiPrefix+"/foods", a.authenticated(a.listFoods))
	mux.Handle("POST "+apiPrefix+"/foods", a.authenticated(a.createFood))
	mux.Handle("GET "+apiPrefix+"/foods/{id}", a.authenticated(a.getFood))
	mux.Handle("PUT "+apiPrefix+"/foods/{id}", a.authenticated(a.updateFood))
	mux.Handle("DELETE "+apiPrefix+"/foods/{id}", a.authenticated(a.deleteFood))
}

func (a *api) listFoods(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePage(r)
	if err != nil {
		return err
	}
	foods, total, err := a.foods.List(r.Context(), userIDFrom(r.Context()), r.URL.Query().Get("q"), page)
	if err != nil {
		return err
	}
	writeList(w, r, foods, page, total)
	return nil
}

func (a *api) createFood(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeJSON[food.Input](w, r)
	if err != nil {
		return err
	}
	created, err := a.foods.Create(r.Context(), userIDFrom(r.Context()), input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusCreated, created)
	return nil
}

func (a *api) getFood(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	found, err := a.foods.Get(r.Context(), userIDFrom(r.Context()), id)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, found)
	return nil
}

func (a *api) updateFood(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	input, err := decodeJSON[food.Input](w, r)
	if err != nil {
		return err
	}
	updated, err := a.foods.Update(r.Context(), userIDFrom(r.Context()), id, input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, updated)
	return nil
}

func (a *api) deleteFood(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := a.foods.Delete(r.Context(), userIDFrom(r.Context()), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
