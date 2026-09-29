package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const Version = "0.1.0-dev"

func New(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := rand.Text()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		status := http.StatusOK
		var payload any
		route := r.URL.Path
		switch route {
		case "/healthz":
			payload = map[string]any{"status": "ok"}
		case "/readyz":
			payload = map[string]any{"status": "ready", "scope": "http-foundation", "dependency_checks": "not_configured"}
		case "/v1":
			payload = map[string]any{"service": "open-flow", "version": Version, "stage": "foundation", "capabilities": []string{}}
		default:
			route = "unmatched"
			status = http.StatusNotFound
			payload = apiError("not_found", "Route not found", requestID)
		}
		if route != "unmatched" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			status = http.StatusMethodNotAllowed
			w.Header().Set("Allow", "GET, HEAD")
			payload = apiError("method_not_allowed", "Method not allowed", requestID)
		}
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			_ = json.NewEncoder(w).Encode(payload)
		}
		logger.Info("http_request", "request_id", requestID, "method", r.Method, "route", route, "status", status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func apiError(code, message, requestID string) any {
	return map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": requestID}}
}
