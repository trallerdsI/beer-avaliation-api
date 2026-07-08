package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq" // Register the PostgreSQL driver.
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	beerHttp "beer-review-app/internal/beer/delivery/http"
	beerRepository "beer-review-app/internal/beer/repository"
	beerUsecase "beer-review-app/internal/beer/usecase"
	monitoring "beer-review-app/internal/monitoring"
	userHttp "beer-review-app/internal/user/delivery/http"
	userRepository "beer-review-app/internal/user/repository"
	userUsecase "beer-review-app/internal/user/usecase"
	appMetrics "beer-review-app/pkg/metrics"
	middleware "beer-review-app/pkg/middleware"
)

// BuildRouter creates the main HTTP router for the application.
func BuildRouter(db *sql.DB, logger *zap.Logger) http.Handler {
	if logger == nil {
		logger = zap.NewNop()
	}

	beerRepo, err := beerRepository.NewPostgresBeerRepository(db)
	if err != nil {
		log.Fatalf("Erro ao criar repositório de cervejas: %v", err)
	}

	userRepo := userRepository.NewPostgresUserRepository(db)
	beerUsecase := beerUsecase.NewBeerUsecase(beerRepo)
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
	if !appMetrics.IsServerlessRuntime() {
		router.Handle("/metrics", promhttp.Handler())
	}

	return router
}

// InitDBFromEnv initializes the database connection using environment variables.
func InitDBFromEnv() (*sql.DB, error) {
	dbConnString := resolveDBConnString()
	if dbConnString == "" {
		return nil, fmt.Errorf("database connection string is not configured")
	}

	return InitDB(dbConnString)
}

// InitDB opens and configures the PostgreSQL connection pool.
func InitDB(dbConnString string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar com o banco de dados: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	retries := 5
	for retries > 0 {
		err = db.Ping()
		if err == nil {
			break
		}
		retries--
		log.Printf("Tentando conectar ao banco de dados... Tentativas restantes: %d", retries)
		time.Sleep(2 * time.Second)
	}

	if retries == 0 {
		db.Close()
		return nil, fmt.Errorf("banco de dados não está disponível após várias tentativas")
	}

	if err := migrateDB(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func resolveDBConnString() string {
	if value := os.Getenv("DB_CONN_STRING"); value != "" {
		return value
	}
	if value := os.Getenv("DBConnString"); value != "" {
		return value
	}
	if value := viper.GetString("DB_CONN_STRING"); value != "" {
		return value
	}
	if value := viper.GetString("DBConnString"); value != "" {
		return value
	}
	return ""
}

func migrateDB(db *sql.DB) error {
	sqlFiles := []string{
		"migrations/create_users_table.sql",
		"migrations/create_beers_table.sql",
		"migrations/create_comments_table.sql",
		"migrations/create_indexes.sql",
	}

	for _, file := range sqlFiles {
		err := executeSQLFile(db, file)
		if err != nil {
			return fmt.Errorf("erro ao executar migração no arquivo %s: %w", file, err)
		}
	}
	log.Println("Todas as tabelas e índices foram criados com sucesso!")
	return nil
}

func executeSQLFile(db *sql.DB, filePath string) error {
	sqlBytes, err := os.ReadFile(filepath.Clean(filePath))
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

// InitializeVercelHandler returns the shared handler the Vercel function uses.
func InitializeVercelHandler() http.Handler {
	logger, err := zap.NewProduction()
	if err != nil {
		logger = zap.NewNop()
	}

	db, err := InitDBFromEnv()
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}

	return BuildRouter(db, logger)
}
