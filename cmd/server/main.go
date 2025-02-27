package main

import (
	"context"
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"beer-review-app/internal/beer/repository"
	"beer-review-app/internal/beer/usecase"

	"go.uber.org/zap"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	beerHttp "beer-review-app/internal/beer/delivery/http"

	userHttp "beer-review-app/internal/user/delivery/http"
	userRepo "beer-review-app/internal/user/repository"
	userUsecase "beer-review-app/internal/user/usecase"

	middleware "beer-review-app/pkg/middleware"

	monitoring "beer-review-app/internal/monitoring"

	_ "net/http/pprof"

	"golang.org/x/time/rate"

	_ "github.com/lib/pq"
)

var (
	responseTimes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_response_time_seconds",
			Help:    "API response time in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	errorCounts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_error_count",
			Help: "Number of API errors",
		},
		[]string{"method", "endpoint", "status"},
	)

	submissionsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_submission_count",
			Help: "Total number of submissions",
		},
	)
)

func init() {
	prometheus.MustRegister(responseTimes)
	prometheus.MustRegister(errorCounts)
	prometheus.MustRegister(submissionsCount)
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database
	db := initDB(os.Getenv("DB_CONN_STRING"))
	defer db.Close()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_URL"),
	})
	defer redisClient.Close()

	// Initialize repositories
	beerRepo, err := repository.NewPostgresBeerRepository(db)
	if err != nil {
		logger.Fatal("Failed to create beer repository", zap.Error(err))
	}
	userRepo := userRepo.NewPostgresUserRepository(db)

	// Initialize usecases
	beerUsecase := usecase.NewBeerUsecase(beerRepo, redisClient)
	userUsecase := userUsecase.NewUserUsecase(userRepo)

	// Initialize controllers
	beerController := beerHttp.NewBeerController(beerUsecase, logger)
	userController := userHttp.NewUserController(userUsecase, logger)
	monitoringController := monitoring.NewMonitoringController(beerUsecase, userUsecase, logger)

	// Set up router with middleware
	router := mux.NewRouter()
	router.Use(middleware.MetricsMiddleware)
	router.Use(middleware.RequestIDMiddleware)

	// API routes
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Beer routes
	apiRouter.HandleFunc("/beers", beerController.GetAllBeers).Methods("GET")
	apiRouter.HandleFunc("/beers", beerController.CreateBeer).Methods("POST")
	apiRouter.HandleFunc("/beers/{id}", beerController.GetBeerByID).Methods("GET")
	apiRouter.HandleFunc("/beers/{id}", beerController.UpdateBeer).Methods("PUT")
	apiRouter.HandleFunc("/beers/{id}", beerController.DeleteBeer).Methods("DELETE")
	apiRouter.HandleFunc("/beers/search", beerController.SearchBeers).Methods("GET")

	// Comment routes
	apiRouter.HandleFunc("/beers/{id}/comments", beerController.AddComment).Methods("POST")
	apiRouter.HandleFunc("/beers/{id}/comments/{commentId}", beerController.DeleteComment).Methods("DELETE")
	apiRouter.HandleFunc("/beers/{id}/comments/{commentId}/like", beerController.LikeComment).Methods("POST")

	// User routes
	apiRouter.HandleFunc("/users/register", userController.Register).Methods("POST")
	apiRouter.HandleFunc("/users/login", userController.Login).Methods("POST")

	// Protected routes
	protected := apiRouter.PathPrefix("/users").Subrouter()
	protected.Use(middleware.AuthMiddleware)
	protected.HandleFunc("/{id}", userController.GetProfile).Methods("GET")
	protected.HandleFunc("/{id}", userController.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/{id}", userController.DeleteAccount).Methods("DELETE")

	// Monitoring routes
	apiRouter.HandleFunc("/stats", monitoringController.GetStats).Methods("GET")
	apiRouter.HandleFunc("/health", monitoringController.HealthCheck).Methods("GET")
	router.Handle("/metrics", promhttp.Handler())

	// Start server
	server := &http.Server{
		Addr:         ":" + os.Getenv("SERVER_PORT"),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}

func rateLimitMiddleware(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(1, 3)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func initDB(dbConnString string) *sql.DB {
	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatal(err)
	}

	// Retry mechanism to wait for the database to be ready
	for {
		err = db.Ping()
		if err == nil {
			break
		}
		log.Println("Waiting for database to be ready... : " + err.Error())
		time.Sleep(5 * time.Second)
	}

	sqlFiles := []string{
		"migrations/create_users_table.sql",
		"migrations/create_beers_table.sql",
		"migrations/create_comments_table.sql",
		"migrations/create_indexes.sql",
	}

	for _, file := range sqlFiles {
		err := executeSQLFile(db, file)
		if err != nil {
			log.Fatalf("Error executing file %s: %v", file, err)
		}
	}

	log.Println("All tables and indexes created successfully!")
	return db
}

// executeSQLFile reads and executes the SQL file
func executeSQLFile(db *sql.DB, filePath string) error {
	sqlBytes, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return err
	}

	_, err = db.Exec(string(sqlBytes))
	return err
}
