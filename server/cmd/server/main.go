package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/config"
	"pbs_help_desk/internal/db"
	"pbs_help_desk/internal/handlers"
	"pbs_help_desk/internal/middleware"
	"pbs_help_desk/internal/repositories"
	"pbs_help_desk/internal/services"
)

const serviceName = "pbs-help-desk-server"

type dependencies struct {
	db           handlers.DBPinger
	authService  *services.AuthService
	tokenManager *auth.TokenManager
	cookieDomain string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.ConnectAndMigrate(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Keep Supabase database active to avoid Free Tier inactivity pauses.
	db.StartKeepAlive(ctx, pool, 6*time.Hour)

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	userRepo := repositories.NewUserRepository(pool)
	authService := services.NewAuthService(userRepo, tokenManager)

	mux := http.NewServeMux()
	registerRoutes(mux, dependencies{
		db:           pool,
		authService:  authService,
		tokenManager: tokenManager,
		cookieDomain: cfg.CookieDomain,
	})

	handler := middleware.LoggingMiddleware(
		middleware.CORSMiddleware(cfg.AllowedOrigins)(
			middleware.RateLimitMiddleware(mux),
		),
	)

	srv := &http.Server{
		Addr:              cfg.Host + ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("Server started", "port", ":"+cfg.Port, "service", serviceName)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped cleanly")
}

func registerRoutes(mux *http.ServeMux, deps dependencies) {
	healthHandler := handlers.NewHealthHandler(serviceName, deps.db)
	mux.HandleFunc("GET /api/health", healthHandler.Check)

	if deps.authService != nil && deps.tokenManager != nil {
		authHandler := handlers.NewAuthHandler(deps.authService, deps.cookieDomain)
		authRequired := auth.Middleware(deps.tokenManager)

		mux.HandleFunc("POST /api/auth/register", authHandler.Register)
		mux.HandleFunc("POST /api/auth/login", authHandler.Login)
		mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)
		mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
		mux.Handle("GET /api/auth/me", authRequired(http.HandlerFunc(authHandler.Me)))
	}
}
