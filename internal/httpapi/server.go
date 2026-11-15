// Package httpapi exposes the REST API. Handlers only translate between HTTP
// and the feature services; they hold no business rules.
package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"fittrack/internal/auth"
	"fittrack/internal/food"
	"fittrack/internal/meal"
	"fittrack/internal/user"
	"fittrack/internal/workout"
)

const apiPrefix = "/api/v1"

// Deps are the collaborators the API needs from the composition root.
type Deps struct {
	DB     *sql.DB
	Tokens *auth.TokenManager
	Hasher auth.PasswordHasher
	Logger *slog.Logger
	Now    func() time.Time
}

type api struct {
	logger   *slog.Logger
	tokens   *auth.TokenManager
	db       *sql.DB
	users    *user.Service
	foods    *food.Service
	workouts *workout.Service
	meals    *meal.Service
}

// New builds the services from deps and returns the routed handler.
func New(deps Deps) http.Handler {
	users := user.NewService(user.NewRepository(deps.DB), deps.Hasher, deps.Tokens, deps.Now)
	foods := food.NewService(food.NewRepository(deps.DB))
	a := &api{
		logger:   deps.Logger,
		tokens:   deps.Tokens,
		db:       deps.DB,
		users:    users,
		foods:    foods,
		workouts: workout.NewService(workout.NewRepository(deps.DB), users),
		meals:    meal.NewService(meal.NewRepository(deps.DB), foods),
	}
	mux := http.NewServeMux()
	a.registerRoutes(mux)
	return a.recoverPanics(a.logRequests(mux))
}

func (a *api) registerRoutes(mux *http.ServeMux) {
	mux.Handle("GET /healthz", a.handle(a.health))
	// The catch-all keeps unknown routes in the JSON error format instead of
	// the mux's plain-text 404.
	mux.Handle("/", a.handle(a.notFound))

	mux.Handle("POST "+apiPrefix+"/auth/register", a.handle(a.register))
	mux.Handle("POST "+apiPrefix+"/auth/login", a.handle(a.login))
	mux.Handle("GET "+apiPrefix+"/profile", a.authenticated(a.getProfile))
	mux.Handle("PUT "+apiPrefix+"/profile", a.authenticated(a.updateProfile))
	a.registerFoodRoutes(mux)
	a.registerWorkoutRoutes(mux)
	a.registerMealRoutes(mux)
}

func (a *api) health(w http.ResponseWriter, r *http.Request) error {
	if err := a.db.PingContext(r.Context()); err != nil {
		return err
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}

func (a *api) notFound(w http.ResponseWriter, r *http.Request) error {
	writeErrorBody(w, r, http.StatusNotFound, "not_found", "route not found")
	return nil
}
