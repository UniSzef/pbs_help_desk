package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	allowed := []string{"https://helpdesk.pbs.edu.pl"}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CORSMiddleware(allowed)(next)

	// 1. Allowed localhost origin with credentials
	req1 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req1.Header.Set("Origin", "http://localhost:4200")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4200" {
		t.Fatalf("expected allowed origin http://localhost:4200, got %q", rec1.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec1.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("expected Access-Control-Allow-Credentials 'true'")
	}

	// 2. Preflight OPTIONS for configured origin
	req2 := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	req2.Header.Set("Origin", "https://helpdesk.pbs.edu.pl")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec2.Code)
	}

	// 3. Disallowed origin gets no allow-origin header
	req3 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req3.Header.Set("Origin", "https://evil.example.com")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected empty Access-Control-Allow-Origin for disallowed origin, got %q", rec3.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRateLimitMiddleware_AllowsAndBlocks(t *testing.T) {
	mutex.Lock()
	visitors = make(map[string]visitor)
	mutex.Unlock()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimitMiddleware(next)

	for i := 0; i < maxRequests; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d after exceeding limit, got %d", http.StatusTooManyRequests, rec.Code)
	}
}
