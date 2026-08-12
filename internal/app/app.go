package app

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	beerHttp "beer-review-app/internal/beer/delivery/http"
	beerRepository "beer-review-app/internal/beer/repository"
	beerUsecase "beer-review-app/internal/beer/usecase"
	monitoring "beer-review-app/internal/monitoring"
	userHttp "beer-review-app/internal/user/delivery/http"
	userRepository "beer-review-app/internal/user/repository"
	userUsecase "beer-review-app/internal/user/usecase"
	appMetrics "beer-review-app/pkg/metrics"
	middleware "beer-review-app/pkg/middleware"
	"beer-review-app/pkg/realtime"
	"beer-review-app/pkg/storage"
)

//go:embed all:migrations/*.sql
var embeddedMigrations embed.FS

//go:embed openapi.yaml
var embeddedOpenAPI embed.FS

func BuildRouter(db *sql.DB, logger *slog.Logger) http.Handler {
	return BuildRouterWithDBErr(db, nil, logger)
}

func BuildRouterWithDBErr(db *sql.DB, dbErr error, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if dbErr != nil && db == nil {
		slog.Warn("base de dados indisponível no arranque; rotas de dados retornarão 503", "err", dbErr)
	}

	beerRepo := newBeerRepo(db)
	userRepo := newUserRepo(db)

	eventHub := realtime.NewHub(64)
	beerUsecase := beerUsecase.NewBeerUsecase(beerRepo, eventHub)
	userUsecase := userUsecase.NewUserUsecase(userRepo)

	var uploader storage.Uploader
	if s, err := storage.NewSupabaseStorageFromEnv(); err != nil {
		slog.Warn("storage de mídia não configurado; upload desativado", "err", err)
	} else {
		uploader = s
	}

	if err := userUsecase.SeedAdmin(context.Background()); err != nil {
		slog.Error("falha no seed de admin", "err", err)
	}

	beerController := beerHttp.NewBeerController(beerUsecase, logger, uploader)
	userController := userHttp.NewUserController(userUsecase, logger)
	monitoringController := monitoring.NewMonitoringController(beerUsecase, beerRepo, userRepo, logger, db, dbErr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/beers/enums", beerController.GetEnums)
	mux.HandleFunc("GET /api/v1/beers", beerController.GetAllBeers)
	mux.HandleFunc("POST /api/v1/beers", middleware.Auth(beerController.CreateBeer))
	mux.HandleFunc("GET /api/v1/beers/{id}", beerController.GetBeerByID)
	mux.HandleFunc("PUT /api/v1/beers/{id}", middleware.Auth(beerController.UpdateBeer))
	mux.HandleFunc("DELETE /api/v1/beers/{id}", middleware.Auth(beerController.DeleteBeer))
	mux.HandleFunc("POST /api/v1/beers/{id}/media", middleware.Auth(beerController.UploadBeerMedia))
	mux.HandleFunc("GET /api/v1/beers/search", beerController.SearchBeers)
	mux.HandleFunc("POST /api/v1/beers/{id}/comments", middleware.Auth(beerController.AddComment))
	mux.HandleFunc("DELETE /api/v1/beers/{id}/comments/{commentId}", middleware.Auth(beerController.DeleteComment))
	mux.HandleFunc("POST /api/v1/beers/{id}/comments/{commentId}/like", middleware.Auth(beerController.LikeComment))
	mux.HandleFunc("GET /api/v1/feed", beerController.GetHomeFeed)
	mux.HandleFunc("GET /api/v1/stream", realtime.SSEHandler(eventHub))
	mux.HandleFunc("POST /api/v1/users/register", userController.Register)
	mux.HandleFunc("POST /api/v1/users/login", userController.Login)
	mux.HandleFunc("POST /api/v1/users/oauth", userController.OAuth)
	mux.HandleFunc("GET /api/v1/users/{id}", middleware.Auth(userController.GetProfile))
	mux.HandleFunc("PUT /api/v1/users/{id}", middleware.Auth(userController.UpdateProfile))
	mux.HandleFunc("DELETE /api/v1/users/{id}", middleware.Auth(userController.DeleteAccount))
	mux.HandleFunc("POST /api/v1/users/{id}/push/subscribe", middleware.Auth(userController.SubscribePush))
	mux.HandleFunc("POST /api/v1/users/{id}/push/unsubscribe", middleware.Auth(userController.UnsubscribePush))
	mux.HandleFunc("GET /api/v1/users/{id}/push", middleware.Auth(userController.ListPushSubscriptions))
	mux.HandleFunc("GET /api/v1/stats", monitoringController.GetStats)
	mux.HandleFunc("GET /api/v1/admin/stats", middleware.RequireAdmin(monitoringController.GetAdminStats))
	mux.HandleFunc("GET /api/v1/users/me/stats", middleware.Auth(monitoringController.GetUserStats))
	mux.HandleFunc("GET /api/v1/health", monitoringController.HealthCheck)
	mux.HandleFunc("GET /docs", docsHandler())
	mux.HandleFunc("GET /docs/openapi.yaml", openapiSpecHandler())
	if !appMetrics.IsServerlessRuntime() {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	handler := http.Handler(mux)
	handler = middleware.CORSMiddleware(handler)
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.CompressionMiddleware(handler)
	handler = middleware.NewRateLimitMiddleware(10, time.Minute)(handler)

	return handler
}

func newBeerRepo(db *sql.DB) beerRepository.BeerRepository {
	if db == nil {
		return nil
	}
	repo, err := beerRepository.NewPostgresBeerRepository(db)
	if err != nil {
		slog.Error("repositório de cervejas indisponível; rotas de dados retornarão 503", "err", err)
		return nil
	}
	return repo
}

func newUserRepo(db *sql.DB) userRepository.UserRepository {
	if db == nil {
		return nil
	}
	repo, err := userRepository.NewPostgresUserRepository(db)
	if err != nil {
		slog.Error("repositório de utilizadores indisponível; rotas de dados retornarão 503", "err", err)
		return nil
	}
	return repo
}

func InitDBFromEnv() (*sql.DB, error) {
	dsn := resolveDBConnString()
	if dsn == "" {
		return nil, fmt.Errorf("database connection string is not configured")
	}
	slog.Info("resolvendo connection string para o banco de dados", "conn", maskPassword(dsn))
	return InitDB(dsn)
}

func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar com o banco de dados: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns())
	db.SetMaxIdleConns(maxIdleConns())
	db.SetConnMaxLifetime(5 * time.Minute)

	if appMetrics.IsServerlessRuntime() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = db.PingContext(ctx); err != nil {
			db.Close()
			slog.Error("falha no ping do banco de dados (serverless)", "err", err, "conn", maskPassword(dsn))
			return nil, fmt.Errorf("banco de dados indisponível: %w", err)
		}
	} else {
		retries := 5
		for retries > 0 {
			err = db.Ping()
			if err == nil {
				break
			}
			retries--
			slog.Warn("tentando conectar ao banco de dados", "retries_left", retries, "err", err)
			time.Sleep(2 * time.Second)
		}
		if retries == 0 {
			db.Close()
			slog.Error("banco de dados indisponível após várias tentativas", "conn", maskPassword(dsn))
			return nil, fmt.Errorf("banco de dados não está disponível após várias tentativas")
		}
	}

	if err := migrateDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrations failed: %w", err)
	}
	return db, nil
}

func maskPassword(connString string) string {
	if i := strings.Index(connString, "://"); i >= 0 {
		prefix := connString[:i+3]
		rest := connString[i+3:]
		if j := strings.Index(rest, "@"); j >= 0 {
			return prefix + "****@" + rest[j+1:]
		}
	}
	return "***masked***"
}

func resolveDBConnString() string {
	for _, key := range []string{"DB_CONN_STRING", "DBConnString"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	for _, key := range []string{"POSTGRES_URL_NON_POOLING", "POSTGRES_URL"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	user := firstNonEmpty(os.Getenv("DB_USER"), os.Getenv("POSTGRES_USER"))
	pass := firstNonEmpty(os.Getenv("DB_PASSWORD"), os.Getenv("POSTGRES_PASSWORD"))
	host := firstNonEmpty(os.Getenv("DB_HOST"), os.Getenv("POSTGRES_HOST"), "localhost")
	port := firstNonEmpty(os.Getenv("DB_PORT"), "5432")
	name := firstNonEmpty(os.Getenv("DB_NAME"), os.Getenv("POSTGRES_DATABASE"), "postgres")
	sslmode := firstNonEmpty(os.Getenv("DB_SSLMODE"), os.Getenv("DB_SSL_MODE"), "require")

	if user == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user), url.QueryEscape(pass), host, port, name, sslmode)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func maxOpenConns() int {
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10
}

func maxIdleConns() int {
	if v := os.Getenv("DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 5
}

// resetSchemaEnabled devolve true se a API deve recriar o esquema do zero a
// cada arranque (DROP + CREATE). Controlado por DB_RESET_SCHEMA (default true).
// Enquanto o banco for descartável (sem front dependiente nem dados definitivos)
// isto mantém o esquema sempre sincronizado com as migrations. Quando o banco
// passar a ter dados reais, basta definir DB_RESET_SCHEMA=false na Vercel para
// desativar o reset destrutivo e passar a aplicar apenas as migrations
// incrementais (idempotentes com IF NOT EXISTS / IF EXISTS).
func resetSchemaEnabled() bool {
	v := strings.ToLower(os.Getenv("DB_RESET_SCHEMA"))
	return v != "false" && v != "0" && v != "no"
}

func migrateDB(db *sql.DB) error {
	// Reset destrutivo em primeiro lugar, mas apenas se ativado.
	sqlFiles := make([]string, 0, 13)
	if resetSchemaEnabled() {
		sqlFiles = append(sqlFiles, "migrations/000_reset.sql")
		slog.Info("DB_RESET_SCHEMA=true: esquema será recriado do zero (banco descartável)")
	} else {
		slog.Warn("DB_RESET_SCHEMA=false: reset destrutivo desativado; a aplicar apenas migrations incrementais (não destrutivas)")
	}

	sqlFiles = append(sqlFiles,
		"migrations/create_users_table.sql",
		"migrations/add_role_to_users.sql",
		"migrations/add_oauth_provider_to_users.sql",
		"migrations/create_beers_table.sql",
		"migrations/add_created_by_to_beers.sql",
		"migrations/add_created_at_to_beers.sql",
		"migrations/add_comments_jsonb.sql",
		"migrations/create_indexes.sql",
		"migrations/use_uuid_pk.sql",
		"migrations/add_updated_at.sql",
		"migrations/extend_media.sql",
		"migrations/drop_legacy_comments_table.sql",
		"migrations/enable_rls.sql",
		"migrations/create_push_subscriptions.sql",
	)

	for _, file := range sqlFiles {
		err := executeSQLFile(db, file)
		if err != nil {
			return fmt.Errorf("erro ao executar migração no arquivo %s: %w", file, err)
		}
	}
	slog.Info("todas as tabelas e índices foram criados com sucesso")
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
	return embeddedMigrations.ReadFile(filepath.ToSlash(filepath.Join("migrations", filepath.Base(filePath))))
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

func InitializeVercelHandler() http.Handler {
	// No Vercel o handler default de slog pode não capturar Info/Warn
	// conforme esperado; forçamos JSON para stdout com nível Debug para que o
	// arranque (resolução de DSN, erro de ping) seja sempre visível nos logs.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	db, err := InitDBFromEnv()
	if err != nil {
		// Error (e nao Warn) para garantir visibilidade no Vercel.
		slog.Error("vercel bootstrap: banco indisponível; rotas de dados retornarão 503", "err", err)
	} else {
		slog.Info("vercel bootstrap: banco inicializado com sucesso")
	}

	return BuildRouterWithDBErr(db, err, logger)
}
