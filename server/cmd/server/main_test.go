package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pbs_help_desk/internal/auth"
	"pbs_help_desk/internal/services"
)

func TestRegisterRoutes(t *testing.T) {
	tm := auth.NewTokenManager("test-secret-key-1234", 15*time.Minute, 24*time.Hour)
	authSvc := services.NewAuthService(nil, tm)

	mux := http.NewServeMux()
	registerRoutes(mux, dependencies{
		authService:  authSvc,
		tokenManager: tm,
	})

	reqHealth := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recHealth := httptest.NewRecorder()
	mux.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Fatalf("expected status %d on /api/health, got %d", http.StatusOK, recHealth.Code)
	}

	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recMe := httptest.NewRecorder()
	mux.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d on unauthenticated /api/auth/me, got %d", http.StatusUnauthorized, recMe.Code)
	}
}
