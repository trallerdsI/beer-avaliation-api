package middleware

import (
	"net/http"
	"time"

	"beer-review-app/pkg/metrics"
)

// MetricsMiddleware wraps an HTTP handler to collect metrics about each request,
// such as the path, method, status code, and response duration.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Call the next handler in the chain
		next.ServeHTTP(rw, r)

		// Record duration and send metrics
		duration := time.Since(start).Seconds()
		metrics.RecordMetrics(r.URL.Path, r.Method, http.StatusText(rw.status), duration)
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
