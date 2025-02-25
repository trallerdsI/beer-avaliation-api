package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

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

func RecordMetrics(path, method, status string, duration float64) {
	RequestDuration.WithLabelValues(path, method, status).Observe(duration)
	TotalRequests.WithLabelValues(path, method, status).Inc()
} 