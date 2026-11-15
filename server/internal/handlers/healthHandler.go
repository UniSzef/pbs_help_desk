package handlers

import (
	"context"
	"net/http"
	"time"
)

// DBPinger verifies database connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthResponse represents the JSON payload returned by the healthcheck endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Database  string `json:"database,omitempty"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
}

// HealthHandler handles healthcheck HTTP requests.
type HealthHandler struct {
	serviceName string
	startedAt   time.Time
	db          DBPinger
}

// NewHealthHandler creates a new HealthHandler instance.
func NewHealthHandler(serviceName string, db DBPinger) *HealthHandler {
	return &HealthHandler{
		serviceName: serviceName,
		startedAt:   time.Now().UTC(),
		db:          db,
	}
}

// Check responds with the current operational status of the server and database.
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

	if h.db != nil {
		pingCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := h.db.Ping(pingCtx); err != nil {
			resp.Status = "degraded"
			resp.Database = "unreachable"
			writeJSON(w, http.StatusServiceUnavailable, resp)
			return
		}
		resp.Database = "ok"
	}

	writeJSON(w, http.StatusOK, resp)
}
