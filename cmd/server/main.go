package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	beerHttp "beer-review-app/internal/beer/delivery/http"
	"beer-review-app/internal/beer/repository"
	"beer-review-app/internal/beer/usecase"
	monitoring "beer-review-app/internal/monitoring"
	userHttp "beer-review-app/internal/user/delivery/http"
	userRepo "beer-review-app/internal/user/repository"
	userUsecase "beer-review-app/internal/user/usecase"
	middleware "beer-review-app/pkg/middleware"
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

	db := initDB(dbConnString)
	defer db.Close()

	beerRepo, err := repository.NewPostgresBeerRepository(db)
	if err != nil {
		log.Fatalf("Erro ao criar repositório de cervejas: %v", err)
	}

	userRepo := userRepo.NewPostgresUserRepository(db)
	beerUsecase := usecase.NewBeerUsecase(beerRepo)
	userUsecase := userUsecase.NewUserUsecase(userRepo)

	beerController := beerHttp.NewBeerController(beerUsecase, logger)
	userController := userHttp.NewUserController(userUsecase, logger)
	monitoringController := monitoring.NewMonitoringController(beerUsecase, userUsecase, logger)

	router := mux.NewRouter()
	router.Use(middleware.MetricsMiddleware)
	router.Use(middleware.RequestIDMiddleware)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/beers", beerController.GetAllBeers).Methods("GET")
	apiRouter.HandleFunc("/beers", beerController.CreateBeer).Methods("POST")
	apiRouter.HandleFunc("/beers/{id}", beerController.GetBeerByID).Methods("GET")
	apiRouter.HandleFunc("/beers/{id}", beerController.UpdateBeer).Methods("PUT")
	apiRouter.HandleFunc("/beers/{id}", beerController.DeleteBeer).Methods("DELETE")
	apiRouter.HandleFunc("/beers/search", beerController.SearchBeers).Methods("GET")

	apiRouter.HandleFunc("/beers/{id}/comments", beerController.AddComment).Methods("POST")
	apiRouter.HandleFunc("/beers/{id}/comments/{commentId}", beerController.DeleteComment).Methods("DELETE")
	apiRouter.HandleFunc("/beers/{id}/comments/{commentId}/like", beerController.LikeComment).Methods("POST")

	apiRouter.HandleFunc("/users/register", userController.Register).Methods("POST")
	apiRouter.HandleFunc("/users/login", userController.Login).Methods("POST")

	protected := apiRouter.PathPrefix("/users").Subrouter()
	protected.Use(middleware.AuthMiddleware)
	protected.HandleFunc("/{id}", userController.GetProfile).Methods("GET")
	protected.HandleFunc("/{id}", userController.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/{id}", userController.DeleteAccount).Methods("DELETE")

	apiRouter.HandleFunc("/stats", monitoringController.GetStats).Methods("GET")
	apiRouter.HandleFunc("/health", healthCheckHandler(db)).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

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

func initDB(dbConnString string) *sql.DB {
	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatalf("Erro ao conectar com o banco de dados: %v", err)
	}

	retries := 5
	for retries > 0 {
		err = db.Ping()
		if err == nil {
			break
		}
		retries--
		log.Printf("Tentando conectar ao banco de dados... Tentativas restantes: %d", retries)
		time.Sleep(5 * time.Second)
	}

	if retries == 0 {
		log.Fatalf("Banco de dados não está disponível após várias tentativas.")
	}

	migrateDB(db)

	return db
}

func migrateDB(db *sql.DB) {
	sqlFiles := []string{
		"migrations/create_users_table.sql",
		"migrations/create_beers_table.sql",
		"migrations/create_comments_table.sql",
		"migrations/create_indexes.sql",
	}

	for _, file := range sqlFiles {
		err := executeSQLFile(db, file)
		if err != nil {
			log.Fatalf("Erro ao executar migração no arquivo %s: %v", file, err)
		}
	}
	log.Println("Todas as tabelas e índices foram criados com sucesso!")
}

func executeSQLFile(db *sql.DB, filePath string) error {
	sqlBytes, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo SQL: %v", err)
	}

	_, err = db.Exec(string(sqlBytes))
	return err
}

func healthCheckHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "Banco de dados não disponível", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
