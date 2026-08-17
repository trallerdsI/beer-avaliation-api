package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Os rótulos usam o PADRÃO ESTÁTICO da rota (ex: "/api/v1/beers/{id}"),
// NUNCA o path com IDs concretos. Isto é a defesa central contra alta
// cardinalidade no Prometheus (Pilar 4): um label "route" com IDs dinâmicos
// explodiria a série temporal e exauriria memória do servidor de monitorização.
var (
	// RequestDuration regista a duração das requisições por rota, método e status.
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method", "status"},
	)

	// TotalRequests conta as requisições por rota, método e status.
	TotalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"route", "method", "status"},
	)
)

// RecordMetrics regista (ou loga, em serverless) as métricas de requisição.
// `pattern` DEVE ser o template estático da rota (r.Pattern), não o path com IDs.
func RecordMetrics(ctx context.Context, pattern, method, status string, duration float64) {
	if IsServerlessRuntime() {
		// Log estruturado determinístico (sem alta cardinalidade): usa o padrão
		// estático da rota, não o path com IDs.
		slog.InfoContext(ctx, "http_request",
			"pattern", pattern,
			"method", method,
			"status", status,
			"duration_seconds", duration,
		)
		return
	}

	RequestDuration.WithLabelValues(pattern, method, status).Observe(duration)
	TotalRequests.WithLabelValues(pattern, method, status).Inc()
}

// IsServerlessRuntime devolve true quando a aplicação corre em modo serverless.
func IsServerlessRuntime() bool {
	return os.Getenv("VERCEL") != "" || os.Getenv("NOW_REGION") != "" || os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

var (
	DBConnectionsOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_db_connections_open",
		Help: "Current number of open connections in the pool",
	})
	DBConnectionsInUse = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_db_connections_in_use",
		Help: "Current number of connections actively executing queries",
	})
	DBConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_db_connections_idle",
		Help: "Current number of idle connections in the pool",
	})
	DBWaitCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "go_sql_db_wait_count_total",
		Help: "Total number of requests that waited for a free connection",
	})
)

// RecordDBStats updates Prometheus gauges/counters from database/sql stats.
func RecordDBStats(stats sql.DBStats) {
	DBConnectionsOpen.Set(float64(stats.OpenConnections))
	DBConnectionsInUse.Set(float64(stats.InUse))
	DBConnectionsIdle.Set(float64(stats.Idle))
	DBWaitCount.Add(float64(stats.WaitCount))
}
