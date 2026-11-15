package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_Check(t *testing.T) {
	handler := NewHealthHandler("pbs-help-desk-server")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("expected status ok, got %q", body.Status)
	}
	if body.Service != "pbs-help-desk-server" {
		t.Errorf("expected service pbs-help-desk-server, got %q", body.Service)
	}
	if body.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if body.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestHealthHandler_Check_MethodNotAllowed(t *testing.T) {
	handler := NewHealthHandler("pbs-help-desk-server")

	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
