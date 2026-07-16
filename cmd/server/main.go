package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/viper"

	"beer-review-app/internal/app"
)

func main() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Warn("não foi possível ler .env", "err", err)
		}
	}

	// Go 1.26: log estruturado nativo via log/slog.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	serverPort := viper.GetString("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8082"
	}

	var db *sql.DB
	db, err := app.InitDBFromEnv()
	if err != nil {
		slog.Error("inicialização sem banco", "err", err)
	}

	// O router é sempre construído. Se o DB não estiver disponível, as rotas
	// de dados retornam 503 específico, mas /docs, /health e /metrics continuam
	// operacionais para diagnóstico (em vez de um 503 global em tudo).
	var router http.Handler = app.BuildRouter(db, logger)

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("erro ao iniciar o servidor", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("erro ao desligar o servidor", "err", err)
	}

	slog.Info("servidor finalizado")
}
