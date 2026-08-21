package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"

	"beer-review-app/pkg/metrics"
)

func TestMetricsEndpoint_ExposesRequiredAlertMetrics(t *testing.T) {
	metrics.RegisterSystemCollectors(prometheus.DefaultRegisterer)

	metrics.DBSqlOpenConns.Set(10)
	metrics.DBSqlInUseConns.Set(2)
	metrics.TotalRequests.WithLabelValues("/api/v1/beers", "GET", "200").Inc()
	metrics.RequestDuration.WithLabelValues("/api/v1/beers", "GET", "200").Observe(0.05)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler := promhttp.Handler()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	requiredMetrics := []string{
		"go_goroutines",
		"go_sql_open_connections",
		"go_sql_in_use_connections",
		"http_requests_total",
		"process_cpu_seconds_total",
	}

	for _, metric := range requiredMetrics {
		assert.Truef(t, strings.Contains(body, metric), "A métrica %s não foi encontrada no endpoint /metrics", metric)
	}
}
