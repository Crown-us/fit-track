package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const bearerPrefix = "Bearer "

type contextKey struct{}

// userIDFrom returns the authenticated user ID stored by authenticate.
func userIDFrom(ctx context.Context) int64 {
	userID, _ := ctx.Value(contextKey{}).(int64)
	return userID
}

// handlerFunc is an http.HandlerFunc that returns its error so the mapping to
// a JSON error response lives in one place.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (a *api) handle(next handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := next(w, r); err != nil {
			a.writeError(w, r, err)
		}
	})
}

// authenticated wraps next so it only runs for requests with a valid token.
func (a *api) authenticated(next handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, bearerPrefix) {
			writeErrorBody(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		userID, err := a.tokens.Parse(strings.TrimPrefix(header, bearerPrefix))
		if err != nil {
			writeErrorBody(w, r, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, userID)
		a.handle(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder captures the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (a *api) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		a.logger.InfoContext(r.Context(), "http request",
			"method", r.Method, "path", r.URL.Path,
			"status", recorder.status, "duration_ms", time.Since(startedAt).Milliseconds())
	})
}

// recoverPanics turns a handler panic into a 500 instead of a dropped
// connection, and keeps the stack-less detail out of the response.
func (a *api) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.ErrorContext(r.Context(), "handler panic", "panic", recovered, "path", r.URL.Path)
				writeErrorBody(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
