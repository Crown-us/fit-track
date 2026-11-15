package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"fittrack/internal/domain"
)

// maxBodyBytes bounds request bodies; every payload in this API is small.
const maxBodyBytes = 1 << 20

// errMalformedRequest marks requests the server could not even parse.
var errMalformedRequest = errors.New("malformed request")

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Details []domain.FieldError `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// The status line is already sent, so logging is all that is left.
		slog.ErrorContext(r.Context(), "write response", "error", err)
	}
}

func writeErrorBody(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, r, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeError maps a service error to the API's single error format.
func (a *api) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, r, http.StatusUnprocessableEntity, errorBody{Error: errorDetail{
			Code: "validation_failed", Message: "one or more fields are invalid", Details: validation.Fields,
		}})
	case errors.Is(err, errMalformedRequest):
		writeErrorBody(w, r, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		writeErrorBody(w, r, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, domain.ErrForbidden):
		writeErrorBody(w, r, http.StatusForbidden, "forbidden", "you cannot modify this resource")
	case errors.Is(err, domain.ErrNotFound):
		writeErrorBody(w, r, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrProfileRequired):
		writeErrorBody(w, r, http.StatusConflict, "profile_required", "complete your profile first")
	case errors.Is(err, domain.ErrConflict):
		writeErrorBody(w, r, http.StatusConflict, "conflict", "resource already exists or is still in use")
	default:
		a.logger.ErrorContext(r.Context(), "request failed", "error", err, "path", r.URL.Path)
		writeErrorBody(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// decodeJSON reads a JSON body into T, rejecting unknown fields so client
// typos surface as errors instead of being silently ignored.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var payload T
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("%w: invalid JSON body: %s", errMalformedRequest, err.Error())
	}
	return payload, nil
}
