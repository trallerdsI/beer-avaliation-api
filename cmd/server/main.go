package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"beer-review-app/internal/app"
	"beer-review-app/pkg/database"
	"beer-review-app/pkg/telemetry"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8082"
	}

	var db *database.RetryableDB
	db, err := app.InitDBFromEnv()
	if err != nil {
		slog.Error("inicialização sem banco", "err", err)
	}

	// O router é sempre construído. Se o DB não estiver disponível, as rotas
	// de dados retornam 503 específico, mas /docs, /health e /metrics continuam
	// operacionais para diagnóstico (em vez de um 503 global em tudo).
	var router http.Handler = app.BuildRouter(db, logger)

	var tracerProvider *sdktrace.TracerProvider
	if os.Getenv("OTEL_TRACES_EXPORTER") != "none" {
		tp, err := telemetry.InitTracerProvider("beer-avaliation-api")
		if err != nil {
			slog.Warn("failed to init OpenTelemetry tracer", "err", err)
		} else {
			tracerProvider = tp
		}
	}

	handler := router
	if tracerProvider != nil {
		handler = otelhttp.NewHandler(router, "beer-avaliation-api",
			otelhttp.WithPropagators(otel.GetTextMapPropagator()),
		)
	}

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("erro ao iniciar o servidor", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Iniciando graceful shutdown do servidor...")

	app.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("erro ao desligar o servidor", "err", err)
	}

	slog.Info("servidor finalizado")

	if tracerProvider != nil {
		telemetry.Shutdown(tracerProvider)
	}
}
