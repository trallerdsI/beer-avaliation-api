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
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	beerHttp "beer-review-app/internal/beer/delivery/http"
	beerRepository "beer-review-app/internal/beer/repository"
	beerUsecasePkg "beer-review-app/internal/beer/usecase"
	monitoring "beer-review-app/internal/monitoring"
	userHttp "beer-review-app/internal/user/delivery/http"
	userRepository "beer-review-app/internal/user/repository"
	userUsecase "beer-review-app/internal/user/usecase"
	"beer-review-app/pkg/database"
	appMetrics "beer-review-app/pkg/metrics"
	middleware "beer-review-app/pkg/middleware"
	"beer-review-app/pkg/moderation"

	"beer-review-app/pkg/events"

	"github.com/redis/go-redis/v9"
)

//go:embed all:migrations/*.sql
var embeddedMigrations embed.FS

//go:embed openapi.yaml
var embeddedOpenAPI embed.FS

var (
	backgroundTasks sync.WaitGroup
	shutdownOnce    sync.Once
	shutdownCtx     context.Context
	shutdownCancel  context.CancelFunc
	redisClient     *redis.Client
	redisMu         sync.Mutex
)

func init() {
	shutdownCtx, shutdownCancel = context.WithCancel(context.Background())
}

// ShutdownContext returns a context that is canceled when the application
// begins graceful shutdown. Background goroutines should select on this
// context to exit cleanly.
func ShutdownContext() context.Context {
	return shutdownCtx
}

// Shutdown signals all background goroutines to stop and waits for them
// to exit. Call this before closing database/redis connections.
func Shutdown() {
	shutdownOnce.Do(func() {
		if shutdownCancel != nil {
			shutdownCancel()
		}
	})
	backgroundTasks.Wait()
}

func CloseRedis() error {
	redisMu.Lock()
	defer redisMu.Unlock()
	if redisClient != nil {
		client := redisClient
		redisClient = nil
		return client.Close()
	}
	return nil
}

// RegisterBackgroundTask increments the WaitGroup counter for a background
// goroutine. The caller must call Done when the goroutine exits.
func RegisterBackgroundTask() {
	backgroundTasks.Add(1)
}

// BackgroundTaskDone decrements the WaitGroup counter. Call this when a
// background goroutine exits.
func BackgroundTaskDone() {
	backgroundTasks.Done()
}

func BuildRouter(db *database.RetryableDB, logger *slog.Logger) http.Handler {
	return BuildRouterWithDBErr(db, nil, logger)
}

func BuildRouterWithDBErr(db *database.RetryableDB, dbErr error, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if dbErr != nil && db == nil {
		slog.Warn("base de dados indisponível no arranque; rotas de dados retornarão 503", "err", dbErr)
	}

	beerRepo := newBeerRepo(db)
	userRepo := newUserRepo(db)

	moderator := newModerator()

	var eventPub events.EventStore
	if db != nil {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL != "" {
			opt, err := redis.ParseURL(redisURL)
			if err == nil {
				client := redis.NewClient(opt)
				pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err = client.Ping(pingCtx).Err()
				cancel()
				if err == nil {
					redisMu.Lock()
					if redisClient != nil {
						_ = redisClient.Close()
					}
					redisClient = client
					redisMu.Unlock()
					redisStore := events.NewRedisStore(client, "beer-api", 72*time.Hour)
					pgStore := events.NewPostgresStore(db.DB)
					eventPub = events.NewCompositeStore(redisStore, pgStore)
				} else {
					_ = client.Close()
				}
			}
		}
	}

	beerUsecase := beerUsecasePkg.NewBeerUsecase(beerRepo, moderator, eventPub)
	moderationRepo, err := beerRepository.NewPostgresModerationRepository(db)
	if err != nil {
		slog.Error("falha ao inicializar repositório de moderação", "err", err)
	}
	moderationUsecase := beerUsecasePkg.NewModerationUsecase(moderationRepo, beerRepo)
	userUsecase := userUsecase.NewUserUsecase(userRepo)

	if err := userUsecase.SeedAdmin(context.Background()); err != nil {
		slog.Error("falha no seed de admin", "err", err)
	}

	beerController := beerHttp.NewBeerController(beerUsecase, logger, eventPub)
	userController := userHttp.NewUserController(userUsecase, logger)
	monitoringController := monitoring.NewMonitoringController(beerRepo, userRepo, logger, db, dbErr, redisClient)
	moderationController := beerHttp.NewModerationController(moderationUsecase, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/beers/enums", beerController.GetEnums)
	mux.HandleFunc("GET /api/v1/beers", beerController.GetAllBeers)
	mux.HandleFunc("POST /api/v1/beers", middleware.Auth(beerController.CreateBeer))
	mux.HandleFunc("GET /api/v1/beers/{id}", beerController.GetBeerByID)
	mux.HandleFunc("PUT /api/v1/beers/{id}", middleware.Auth(beerController.UpdateBeer))
	mux.HandleFunc("DELETE /api/v1/beers/{id}", middleware.RequireAdmin(beerController.DeleteBeer))
	mux.HandleFunc("GET /api/v1/beers/search", beerController.SearchBeers)
	mux.HandleFunc("GET /api/v1/beers/{id}/events", middleware.RequireAdmin(beerController.ListBeerEvents))
	mux.HandleFunc("POST /api/v1/beers/{id}/comments", middleware.Auth(beerController.AddComment))
	mux.HandleFunc("DELETE /api/v1/beers/{id}/comments/{commentId}", middleware.Auth(beerController.DeleteComment))
	mux.HandleFunc("POST /api/v1/beers/{id}/comments/{commentId}/like", middleware.Auth(beerController.LikeComment))
	mux.HandleFunc("POST /api/v1/beers/{id}/reports", middleware.Auth(func(w http.ResponseWriter, r *http.Request) {
		middleware.NewRateLimitMiddleware(5, time.Minute)(http.HandlerFunc(moderationController.ReportBeer)).ServeHTTP(w, r)
	}))
	mux.HandleFunc("POST /api/v1/beers/{id}/deletion-requests", middleware.Auth(moderationController.RequestDeletion))
	mux.HandleFunc("GET /api/v1/beers/{id}/reports", middleware.Auth(moderationController.GetBeerReports))
	mux.HandleFunc("GET /api/v1/admin/reports", middleware.RequireAdmin(moderationController.GetReports))
	mux.HandleFunc("PATCH /api/v1/admin/reports/{id}", middleware.RequireAdmin(moderationController.ResolveReport))
	mux.HandleFunc("GET /api/v1/admin/deletion-requests", middleware.RequireAdmin(moderationController.GetDeletionRequests))
	mux.HandleFunc("PATCH /api/v1/admin/deletion-requests/{id}", middleware.RequireAdmin(moderationController.ResolveDeletionRequest))
	mux.HandleFunc("GET /api/v1/feed", beerController.GetHomeFeed)
	mux.HandleFunc("POST /api/v1/users/register", userController.Register)
	mux.HandleFunc("POST /api/v1/users/login", userController.Login)
	mux.HandleFunc("POST /api/v1/users/oauth", userController.OAuth)
	mux.HandleFunc("POST /api/v1/auth/refresh", userController.Refresh)
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
	mux.HandleFunc("GET /healthz", monitoringController.LivenessProbe)
	mux.HandleFunc("GET /readyz", monitoringController.ReadinessProbe)
	mux.HandleFunc("GET /docs", docsHandler())
	mux.HandleFunc("GET /docs/openapi.yaml", openapiSpecHandler())
	if !appMetrics.IsServerlessRuntime() {
		mux.Handle("GET /metrics", newDBMetricsHandler(db, promhttp.Handler()))
	}

	handler := http.Handler(mux)
	handler = middleware.CORSMiddleware(handler)
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.CompressionMiddleware(handler)
	handler = middleware.NewRateLimitMiddleware(10, time.Minute)(handler)

	// Marca o router para que o serverless handler saiba se o pool
	// de DB foi resolvido (via RouterHasDB). Em long-running, sempre
	// será true; em serverless, começa false e vira true após o
	// hot-swap preguiçoso.
	return wrapRouter(handler, db != nil)
}

func newBeerRepo(db *database.RetryableDB) beerRepository.BeerRepository {
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

func newUserRepo(db *database.RetryableDB) userRepository.UserRepository {
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

func newModerator() moderation.Moderator {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if strings.TrimSpace(apiKey) != "" {
		mod, err := moderation.NewOpenAIModerator(apiKey)
		if err == nil {
			return moderation.NewModeratorWithCache(mod)
		}
		slog.Warn("falha ao criar OpenAI Moderator; usando Noop", "err", err)
	}
	slog.Warn("OPENAI_API_KEY não configurada; API rodando com NoopModerator (modo permissivo)")
	return moderation.NewNoopModerator()
}

func InitDBFromEnv() (*database.RetryableDB, error) {
	dsn := resolveDBConnString()
	if dsn == "" {
		return nil, fmt.Errorf("database connection string is not configured")
	}
	slog.Info("resolvendo connection string para o banco de dados", "conn", maskPassword(dsn))
	return InitDB(dsn)
}

func InitDB(dsn string) (*database.RetryableDB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar com o banco de dados: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns())
	db.SetMaxIdleConns(maxIdleConns())
	db.SetConnMaxLifetime(5 * time.Minute)

	retryable := database.NewRetryableDB(db)

	if appMetrics.IsServerlessRuntime() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = db.PingContext(ctx); err != nil {
			_ = db.Close()
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
			_ = db.Close()
			slog.Error("banco de dados indisponível após várias tentativas", "conn", maskPassword(dsn))
			return nil, fmt.Errorf("banco de dados não está disponível após várias tentativas")
		}
	}

	if err := migrateDB(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrations failed: %w", err)
	}
	return retryable, nil
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
	for _, key := range []string{"DATABASE_URL", "DB_CONN_STRING", "DBConnString"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	// Prefer the provider's pooled URL for serverless cold starts. The
	// non-pooling URL remains a fallback for migrations and local setups.
	for _, key := range []string{"POSTGRES_URL", "POSTGRES_URL_NON_POOLING"} {
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
// cada arranque (DROP + CREATE). Controlado por DB_RESET_SCHEMA (default false).
// O reset destrutivo precisa ser explicitamente habilitado em desenvolvimento
// ou testes; sem isso, apenas migrations incrementais são aplicadas.
//
// Invariante de segurança: DB_RESET_SCHEMA=true é TERMINANTEMENTE PROIBIDO
// em produção. O guard abaixo é deliberadamente fail-fast: preferimos
// recusar o arranque a aceitar uma perda silenciosa de dados.
func resetSchemaEnabled() bool {
	v := strings.ToLower(os.Getenv("DB_RESET_SCHEMA"))
	return v == "true" || v == "1" || v == "yes"
}

func guardDestructiveResetInProduction() {
	if !resetSchemaEnabled() {
		return
	}
	env := strings.ToLower(os.Getenv("ENV"))
	if env == "production" || env == "prod" {
		slog.Error("DB_RESET_SCHEMA ativo em produção — operação destrutiva bloqueada",
			"env", env, "db_reset_schema", os.Getenv("DB_RESET_SCHEMA"))
		panic("DB_RESET_SCHEMA=true não é permitido quando ENV=production; defina DB_RESET_SCHEMA=false antes de arrancar")
	}
}

func migrateDB(db *sql.DB) error {
	// Guard obrigatório: nunca executa reset destrutivo em produção.
	guardDestructiveResetInProduction()

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
		"migrations/add_beer_reports.sql",
		"migrations/add_beer_deletion_requests.sql",
		"migrations/add_moderation_rls.sql",
		"migrations/add_beer_fts.sql",
		"migrations/add_beer_trgm_index.sql",
		"migrations/create_beer_events_table.sql",
		"migrations/create_refresh_tokens.sql",
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

func newDBMetricsHandler(db *database.RetryableDB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if db != nil {
			appMetrics.RecordDBStats(db.Stats())
		}
		next.ServeHTTP(w, r)
	})
}
