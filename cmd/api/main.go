// Command api runs the fitness and calorie tracker HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Crown-us/fit-track/internal/auth"
	"github.com/Crown-us/fit-track/internal/config"
	"github.com/Crown-us/fit-track/internal/httpapi"
	"github.com/Crown-us/fit-track/internal/ratelimit"
	"github.com/Crown-us/fit-track/internal/store"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Getenv)
	stop()
	if err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("api", flag.ContinueOnError)
	isMigrateOnly := flags.Bool("migrate-only", false, "apply migrations and seed data, then exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	db, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	if err := prepareDatabase(ctx, db); err != nil {
		return errors.Join(err, db.Close())
	}
	if *isMigrateOnly {
		logger.InfoContext(ctx, "database is up to date")
		return db.Close()
	}

	handler := httpapi.New(httpapi.Deps{
		DB:                db,
		Tokens:            auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL, time.Now),
		Hasher:            auth.NewPasswordHasher(cfg.BcryptCost),
		Logger:            logger,
		Now:               func() time.Time { return time.Now().In(cfg.Location) },
		AuthLimiter:       ratelimit.New(cfg.AuthRateLimit, cfg.AuthRateWindow, time.Now),
		TrustProxyHeaders: cfg.TrustProxyHeaders,
	})
	return errors.Join(serve(ctx, logger, cfg.HTTPAddr, handler), db.Close())
}

// serve runs the HTTP server until ctx is cancelled, then drains in-flight
// requests before returning.
func serve(ctx context.Context, logger *slog.Logger, addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	logger.InfoContext(ctx, "api listening", "addr", addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}
	logger.InfoContext(ctx, "shutting down")
	// The parent context is already cancelled, so the drain gets its own.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http: %w", err)
	}
	return nil
}
