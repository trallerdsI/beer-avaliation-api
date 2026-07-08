package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"beer-review-app/internal/app"
)

func main() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Aviso: não foi possível ler .env: %s", err)
		}
	}

	logger, err := zap.NewProduction()
	if err != nil {
		logger = zap.NewNop()
	}
	defer logger.Sync()

	serverPort := viper.GetString("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8082"
	}

	var db *sql.DB
	db, err = app.InitDBFromEnv()
	if err != nil {
		log.Printf("Inicialização sem banco: %v", err)
	}

	var router http.Handler
	if db != nil {
		router = app.BuildRouter(db, logger)
	} else {
		router = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database is not ready", http.StatusServiceUnavailable)
		})
	}

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Erro ao iniciar o servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Erro ao desligar o servidor: %v", err)
	}

	log.Println("Servidor finalizado")
}
