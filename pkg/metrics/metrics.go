package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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

	// SSEActiveConnections conta conexões SSE ativas no hub realtime.
	SSEActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sse_active_connections",
		Help: "Current number of active SSE connections",
	})
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
	DBSqlOpenConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_open_connections",
		Help: "Número de conexões abertas no pool do banco de dados.",
	})
	DBSqlInUseConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_in_use_connections",
		Help: "Número de conexões em uso no pool do banco de dados.",
	})
	DBSqlIdleConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "go_sql_idle_connections",
		Help: "Número de conexões idle no pool do banco de dados.",
	})
	DBWaitCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "go_sql_wait_count_total",
		Help: "Total de requisições que esperaram por uma conexão livre.",
	})

	ModerationCacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "moderation_cache_hits_total",
		Help: "Total number of moderation cache hits by backend",
	}, []string{"backend"})
)

// RegisterSystemCollectors registra coletores nativos do Go runtime e processo.
// Isso expõe métricas como go_goroutines, go_memstats_*, process_cpu_seconds_total.
func RegisterSystemCollectors(reg prometheus.Registerer) {
	_ = reg.Register(collectors.NewGoCollector())
	_ = reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// RecordDBStats updates Prometheus gauges/counters from database/sql stats.
func RecordDBStats(stats sql.DBStats) {
	DBSqlOpenConns.Set(float64(stats.OpenConnections))
	DBSqlInUseConns.Set(float64(stats.InUse))
	DBSqlIdleConns.Set(float64(stats.Idle))
	DBWaitCount.Add(float64(stats.WaitCount))
}

func IncSSEConnections() {
	SSEActiveConnections.Inc()
}

func DecSSEConnections() {
	SSEActiveConnections.Dec()
}
