package app

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq" // Register the PostgreSQL driver.
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"

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

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

//go:embed openapi.yaml
var embeddedOpenAPI embed.FS

// BuildRouter creates the main HTTP router for the application.
//
// Go 1.22+ Enhanced Routing: net/http.ServeMux nativo suporta método + padrão
// de caminho (ex: "GET /api/v1/beers/{id}"). O roteador expõe r.Pattern com o
// template estático da rota, eliminando cardinalidade no Prometheus e removendo
// a dependência externa gorilla/mux.
func BuildRouter(db *sql.DB, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
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

	// Go 1.22+ ServeMux: registro declarativo com método + padrão.
	mux := http.NewServeMux()

	// Middleware global aplicado via wrapping (RequestID + Metrics + Auth contextual).
	mux.HandleFunc("GET /api/v1/beers", beerController.GetAllBeers)
	mux.HandleFunc("POST /api/v1/beers", beerController.CreateBeer)
	mux.HandleFunc("GET /api/v1/beers/{id}", beerController.GetBeerByID)
	mux.HandleFunc("PUT /api/v1/beers/{id}", beerController.UpdateBeer)
	mux.HandleFunc("DELETE /api/v1/beers/{id}", beerController.DeleteBeer)
	mux.HandleFunc("GET /api/v1/beers/search", beerController.SearchBeers)
	mux.HandleFunc("POST /api/v1/beers/{id}/comments", beerController.AddComment)
	mux.HandleFunc("DELETE /api/v1/beers/{id}/comments/{commentId}", beerController.DeleteComment)
	mux.HandleFunc("POST /api/v1/beers/{id}/comments/{commentId}/like", beerController.LikeComment)

	mux.HandleFunc("POST /api/v1/users/register", userController.Register)
	mux.HandleFunc("POST /api/v1/users/login", userController.Login)

	// Rotas protegidas por JWT.
	mux.HandleFunc("GET /api/v1/users/{id}", middleware.Auth(userController.GetProfile))
	mux.HandleFunc("PUT /api/v1/users/{id}", middleware.Auth(userController.UpdateProfile))
	mux.HandleFunc("DELETE /api/v1/users/{id}", middleware.Auth(userController.DeleteAccount))

	mux.HandleFunc("GET /api/v1/stats", monitoringController.GetStats)
	mux.HandleFunc("GET /api/v1/health", healthCheckHandler(db))

	mux.HandleFunc("GET /docs", docsHandler())
	mux.HandleFunc("GET /docs/openapi.yaml", openapiSpecHandler())
	if !appMetrics.IsServerlessRuntime() {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	// Encadeamento de middlewares: RequestID -> Metrics -> handler.
	var handler http.Handler = mux
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.RequestIDMiddleware(handler)

	return handler
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

	db.SetMaxOpenConns(maxOpenConns())
	db.SetMaxIdleConns(maxIdleConns())
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
		return nil, fmt.Errorf("migrations failed: %w", err)
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

// maxOpenConns lê DB_MAX_OPEN_CONNS (padrão 10) para tuning do pool por ambiente.
func maxOpenConns() int {
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10
}

// maxIdleConns lê DB_MAX_IDLE_CONNS (padrão 5) para tuning do pool por ambiente.
func maxIdleConns() int {
	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 5
}

func migrateDB(db *sql.DB) error {
	sqlFiles := []string{
		"migrations/create_users_table.sql",
		"migrations/create_beers_table.sql",
		"migrations/create_comments_table.sql",
		"migrations/add_comments_jsonb.sql",
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
	sqlBytes, err := readMigrationSQL(filePath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(sqlBytes))
	return err
}

func readMigrationSQL(filePath string) ([]byte, error) {
	if data, err := embeddedMigrations.ReadFile(filepath.ToSlash(filepath.Join("migrations", filepath.Base(filePath)))); err == nil {
		return data, nil
	}

	resolvedPath, err := resolveMigrationPath(filePath)
	if err != nil {
		return nil, err
	}

	sqlBytes, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo SQL: %v", err)
	}
	return sqlBytes, nil
}

func resolveMigrationPath(filePath string) (string, error) {
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		if _, err := os.Stat(cleanPath); err == nil {
			return cleanPath, nil
		}
		return "", fmt.Errorf("migration file not found: %s", cleanPath)
	}

	candidates := []string{
		cleanPath,
		filepath.Join(".", cleanPath),
		filepath.Join("..", cleanPath),
		filepath.Join("..", "..", cleanPath),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if _, file, _, ok := runtime.Caller(0); ok {
		baseDir := filepath.Dir(file)
		for _, candidate := range []string{
			filepath.Join(baseDir, "..", "..", cleanPath),
			filepath.Join(baseDir, "..", cleanPath),
			filepath.Join(baseDir, cleanPath),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve migration path: %w", err)
	}

	if strings.Contains(workingDir, "/api") || strings.Contains(workingDir, "\\api") {
		rootCandidate := filepath.Join(workingDir, "..", cleanPath)
		if _, err := os.Stat(rootCandidate); err == nil {
			return rootCandidate, nil
		}
	}

	return "", fmt.Errorf("migration file not found: %s", cleanPath)
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

func docsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Beer Avaliation API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
    <style>
      body { margin: 0; background: #fafafa; }
      #swagger-ui { max-width: 1200px; margin: 0 auto; padding: 20px; }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
    <script>
      window.onload = () => {
        SwaggerUIBundle({
          url: '/docs/openapi.yaml',
          dom_id: '#swagger-ui',
          deepLinking: true,
          presets: [SwaggerUIBundle.presets.apis],
        });
      };
    </script>
  </body>
</html>`)
	}
}

func openapiSpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		specData, err := embeddedOpenAPI.ReadFile("openapi.yaml")
		if err != nil {
			http.Error(w, "openapi spec not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(specData)
	}
}

// InitializeVercelHandler returns the shared handler the Vercel function uses.
func InitializeVercelHandler() http.Handler {
	logger := slog.Default()

	db, err := InitDBFromEnv()
	if err != nil {
		log.Printf("Vercel bootstrap warning: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database is not ready", http.StatusServiceUnavailable)
		})
	}

	return BuildRouter(db, logger)
}
