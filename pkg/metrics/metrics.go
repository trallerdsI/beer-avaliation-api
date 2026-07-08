package metrics

import (
	"log"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestDuration stores the duration of HTTP requests by route, method, and status.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

	// TotalRequests counts HTTP requests by path, method, and status.
	TotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_users",
			Help: "Number of currently active users",
		},
	)

	BeerCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_beers",
			Help: "Total number of beers in the system",
		},
	)

	CommentCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_comments",
			Help: "Total number of comments in the system",
		},
	)
)

// RecordMetrics logs or records request metrics depending on the runtime environment.
func RecordMetrics(path, method, status string, duration float64) {
	if IsServerlessRuntime() {
		log.Printf(`{"event":"http_request","path":"%s","method":"%s","status":"%s","duration_seconds":%.6f}`, path, method, status, duration)
		return
	}

	RequestDuration.WithLabelValues(path, method, status).Observe(duration)
	TotalRequests.WithLabelValues(path, method, status).Inc()
}

// IsServerlessRuntime returns true when the application is running in serverless mode.
func IsServerlessRuntime() bool {
	return os.Getenv("VERCEL") != "" || os.Getenv("NOW_REGION") != "" || os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}
