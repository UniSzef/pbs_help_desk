package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the JSON payload returned by the healthcheck endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
}

// HealthHandler handles healthcheck HTTP requests.
type HealthHandler struct {
	serviceName string
	startedAt   time.Time
}

// NewHealthHandler creates a new HealthHandler instance.
func NewHealthHandler(serviceName string) *HealthHandler {
	return &HealthHandler{
		serviceName: serviceName,
		startedAt:   time.Now().UTC(),
	}
}

// Check responds with the current operational status of the server.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := time.Now().UTC()
	resp := HealthResponse{
		Status:    "ok",
		Service:   h.serviceName,
		Timestamp: now.Format(time.RFC3339),
		Uptime:    now.Sub(h.startedAt).Round(time.Second).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
