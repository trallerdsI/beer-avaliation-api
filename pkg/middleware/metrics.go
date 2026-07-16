package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"beer-review-app/pkg/metrics"
)

// MetricsMiddleware wraps an HTTP handler to collect metrics about each request,
// such as the route pattern, method, status code, and response duration.
//
// Go 1.22+: usa r.Pattern que retorna o template estático da rota
// (ex: "/api/v1/beers/{id}"), evitando alta cardinalidade no Prometheus.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code.
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Call the next handler in the chain.
		next.ServeHTTP(rw, r)

		// Route pattern estático (sem IDs) para métricas de baixa cardinalidade.
		pattern := r.Pattern
		if pattern == "" {
			pattern = r.URL.Path
		}

		// Record duration and send metrics.
		duration := time.Since(start).Seconds()
		metrics.RecordMetrics(r.Context(), pattern, r.Method, http.StatusText(rw.status), duration)

		// Log estruturado via slog com contexto da requisição.
		slog.InfoContext(r.Context(), "request completed",
			"pattern", pattern,
			"method", r.Method,
			"status", rw.status,
			"duration_ms", duration*1000,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code for metrics logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the HTTP status code before passing it along.
func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
