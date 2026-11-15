// Package httpapi exposes the REST API. Handlers only translate between HTTP
// and the feature services; they hold no business rules.
package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/Crown-us/fit-track/internal/auth"
	"github.com/Crown-us/fit-track/internal/coach"
	"github.com/Crown-us/fit-track/internal/food"
	"github.com/Crown-us/fit-track/internal/meal"
	"github.com/Crown-us/fit-track/internal/ratelimit"
	"github.com/Crown-us/fit-track/internal/summary"
	"github.com/Crown-us/fit-track/internal/user"
	"github.com/Crown-us/fit-track/internal/weight"
	"github.com/Crown-us/fit-track/internal/workout"
)

const apiPrefix = "/api/v1"

// Deps are the collaborators the API needs from the composition root.
type Deps struct {
	DB     *sql.DB
	Tokens *auth.TokenManager
	Hasher auth.PasswordHasher
	Logger *slog.Logger
	Now    func() time.Time
	// AuthLimiter throttles the register and login endpoints per client.
	AuthLimiter *ratelimit.Limiter
	// TrustProxyHeaders is true when a trusted reverse proxy sets
	// X-Forwarded-For; see clientIP.
	TrustProxyHeaders bool
}

type api struct {
	logger    *slog.Logger
	tokens    *auth.TokenManager
	db        *sql.DB
	users     *user.Service
	foods     *food.Service
	workouts  *workout.Service
	meals     *meal.Service
	weights   *weight.Service
	summaries *summary.Service
	coach     coach.Coach

	authLimiter       *ratelimit.Limiter
	trustProxyHeaders bool
}

// New builds the services from deps and returns the routed handler.
func New(deps Deps) http.Handler {
	users := user.NewService(user.NewRepository(deps.DB), deps.Hasher, deps.Tokens, deps.Now)
	foods := food.NewService(food.NewRepository(deps.DB))
	meals := meal.NewService(meal.NewRepository(deps.DB), foods)
	workoutRepo := workout.NewRepository(deps.DB)
	weights := weight.NewService(weight.NewRepository(deps.DB), deps.Now)
	a := &api{
		logger:   deps.Logger,
		tokens:   deps.Tokens,
		db:       deps.DB,
		users:    users,
		foods:    foods,
		workouts: workout.NewService(workoutRepo, weights),
		meals:    meals,
		weights:  weights,
		summaries: summary.NewService(summary.Sources{
			Profiles: users, Meals: meals, Workouts: workoutRepo, Weights: weights,
		}, deps.Now),
		coach: coach.NewRuleBased(),

		authLimiter:       deps.AuthLimiter,
		trustProxyHeaders: deps.TrustProxyHeaders,
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

	mux.Handle("POST "+apiPrefix+"/auth/register", a.rateLimited(a.handle(a.register)))
	mux.Handle("POST "+apiPrefix+"/auth/login", a.rateLimited(a.handle(a.login)))
	mux.Handle("GET "+apiPrefix+"/profile", a.authenticated(a.getProfile))
	mux.Handle("PUT "+apiPrefix+"/profile", a.authenticated(a.updateProfile))
	a.registerFoodRoutes(mux)
	a.registerWorkoutRoutes(mux)
	a.registerMealRoutes(mux)
	a.registerWeightRoutes(mux)
	a.registerSummaryRoutes(mux)
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
