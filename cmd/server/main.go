package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"beer-review-app/internal/app"
)

func main() {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Erro ao ler arquivo de configuração: %s", err)
	}

	// Agora você pode acessar as variáveis do .env
	dbUser := viper.GetString("DB_USER")
	dbPassword := viper.GetString("DB_PASSWORD")
	log.Printf("DB User: %s, DB Password: %s", dbUser, dbPassword)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Erro ao ler arquivo de configuração: %s", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Erro ao criar logger: %s", err)
	}
	defer logger.Sync()

	serverPort := viper.GetString("SERVER_PORT")
	dbConnString := viper.GetString("DB_CONN_STRING")
	if serverPort == "" || dbConnString == "" {
		log.Fatalf("As variáveis de ambiente SERVER_PORT ou DB_CONN_STRING não estão definidas.")
	}

	db := app.InitDB(dbConnString)
	defer db.Close()

	router := app.BuildRouter(db, logger)

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar o servidor: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Erro ao desligar o servidor: %v", err)
	}

	log.Println("Servidor finalizado")
}
