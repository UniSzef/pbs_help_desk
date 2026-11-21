package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockPinger struct {
	err error
}

func (m mockPinger) Ping(_ context.Context) error {
	return m.err
}

func TestHealthHandler_Check(t *testing.T) {
	handler := NewHealthHandler("pbs-help-desk-server", mockPinger{})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json; charset=utf-8, got %q", ct)
	}

	var body HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
	if body.Database != "ok" {
		t.Errorf("expected database ok, got %q", body.Database)
	}
	if body.Service != "pbs-help-desk-server" {
		t.Errorf("expected service pbs-help-desk-server, got %q", body.Service)
	}
	if body.Timestamp == "" || body.Uptime == "" {
		t.Error("expected non-empty timestamp and uptime")
	}
}

func TestHealthHandler_Check_DBUnreachable(t *testing.T) {
	handler := NewHealthHandler("pbs-help-desk-server", mockPinger{err: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHealthHandler_Check_MethodNotAllowed(t *testing.T) {
	handler := NewHealthHandler("pbs-help-desk-server", nil)

	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
