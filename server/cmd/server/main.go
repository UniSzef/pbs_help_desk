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

	"pbs_help_desk/internal/handlers"
	"pbs_help_desk/internal/middleware"

	"github.com/joho/godotenv"
)

const (
	defaultPort = "8080"
	serviceName = "pbs-help-desk-server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file loaded, using environment variables and defaults")
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = defaultPort
	}

	mux := http.NewServeMux()
	registerRoutes(mux)

	handler := middleware.LoggingMiddleware(
		middleware.CORSMiddleware(
			middleware.RateLimitMiddleware(mux),
		),
	)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Server started", "port", ":"+port, "service", serviceName)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped cleanly")
}

func registerRoutes(mux *http.ServeMux) {
	healthHandler := handlers.NewHealthHandler(serviceName)
	mux.HandleFunc("GET /api/health", healthHandler.Check)
}
