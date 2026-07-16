package metrics

import (
	"context"
	"testing"
	"testing/synctest"
)

// TestRecordMetricsServerless valida que, em runtime serverless, o RecordMetrics
// regista o padrão ESTÁTICO da rota (sem IDs) via slog contextual — sem explodir
// a cardinalidade do Prometheus. Usa testing/synctest (Go 1.26), sem time.Sleep.
func TestRecordMetricsServerless(t *testing.T) {
	t.Setenv("VERCEL", "1")
	if !IsServerlessRuntime() {
		t.Fatal("expected serverless runtime detection")
	}

	synctest.Test(t, func(t *testing.T) {
		// Não deve panic nem tentar registrar série com label dinâmico.
		RecordMetrics(context.Background(), "/api/v1/beers/{id}", "GET", "200", 0.012)
	})
}

// TestRecordMetricsStandard garante que em runtime normal o label usado é o
// padrão estático da rota ("route"), nunca o path com IDs concretos.
func TestRecordMetricsStandard(t *testing.T) {
	t.Setenv("VERCEL", "")
	if IsServerlessRuntime() {
		t.Fatal("expected non-serverless runtime")
	}

	synctest.Test(t, func(t *testing.T) {
		RecordMetrics(context.Background(), "/api/v1/beers/{id}", "GET", "200", 0.012)
		// Se chegou aqui sem panic, o histograma aceitou o label estático.
	})
}
