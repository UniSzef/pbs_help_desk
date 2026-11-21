package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	limitDuration = 60 * time.Second
	maxRequests   = 100
)

type visitor struct {
	count      int
	limitStart time.Time
	lastSeen   time.Time
}

var (
	visitors = make(map[string]visitor)
	mutex    sync.Mutex
)

func init() {
	go cleanupVisitors()
}

// statusRecorder wraps http.ResponseWriter to capture the HTTP status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs incoming HTTP requests with method, path, status, and duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", extractClientIP(r.RemoteAddr),
		)
	})
}

// CORSMiddleware configures Cross-Origin Resource Sharing headers with credentials support for HttpOnly cookies.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Requested-With")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowedList []string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}
	for _, allowed := range allowedList {
		if allowed == "*" || strings.EqualFold(allowed, origin) || strings.EqualFold(allowed, host) {
			return true
		}
	}
	return false
}

// RateLimitMiddleware applies a simple per-IP sliding-window request limit.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := extractClientIP(r.RemoteAddr)
		now := time.Now()

		mutex.Lock()
		v, ok := visitors[host]

		if !ok || now.Sub(v.limitStart) >= limitDuration {
			visitors[host] = visitor{
				count:      1,
				limitStart: now,
				lastSeen:   now,
			}
			mutex.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if v.count >= maxRequests {
			mutex.Unlock()
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		v.count++
		v.lastSeen = now
		visitors[host] = v
		mutex.Unlock()

		next.ServeHTTP(w, r)
	})
}

func extractClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		mutex.Lock()
		for ip, v := range visitors {
			if now.Sub(v.lastSeen) >= 5*time.Minute {
				delete(visitors, ip)
			}
		}
		mutex.Unlock()
	}
}
