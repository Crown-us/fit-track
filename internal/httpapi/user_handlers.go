package httpapi

import (
	"errors"
	"net/http"

	"github.com/Crown-us/fit-track/internal/domain"
	"github.com/Crown-us/fit-track/internal/user"
)

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type registerResponse struct {
	User  user.User `json:"user"`
	Token string    `json:"token"`
}

func (a *api) register(w http.ResponseWriter, r *http.Request) error {
	request, err := decodeJSON[credentialsRequest](w, r)
	if err != nil {
		return err
	}
	created, token, err := a.users.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusCreated, registerResponse{User: created, Token: token})
	return nil
}

func (a *api) login(w http.ResponseWriter, r *http.Request) error {
	request, err := decodeJSON[credentialsRequest](w, r)
	if err != nil {
		return err
	}
	token, err := a.users.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, tokenResponse{Token: token})
	return nil
}

func (a *api) getProfile(w http.ResponseWriter, r *http.Request) error {
	profile, err := a.users.Profile(r.Context(), userIDFrom(r.Context()))
	if errors.Is(err, domain.ErrNotFound) {
		// The account exists; only the profile is missing. Say so precisely.
		return domain.ErrProfileRequired
	}
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, profile)
	return nil
}

func (a *api) updateProfile(w http.ResponseWriter, r *http.Request) error {
	input, err := decodeJSON[user.ProfileInput](w, r)
	if err != nil {
		return err
	}
	profile, err := a.users.UpdateProfile(r.Context(), userIDFrom(r.Context()), input)
	if err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, profile)
	return nil
}
