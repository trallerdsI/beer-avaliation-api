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
	"beer-review-app/pkg/realtime"
	"beer-review-app/pkg/storage"
)

//go:embed all:migrations/*.sql
var embeddedMigrations embed.FS

//go:embed openapi.yaml
var embeddedOpenAPI embed.FS

// BuildRouter creates the main HTTP router for the application.
//
// Go 1.22+ Enhanced Routing: net/http.ServeMux nativo suporta método + padrão
// de caminho (ex: "GET /api/v1/beers/{id}"). O roteador expõe r.Pattern com o
// template estático da rota, eliminando cardinalidade no Prometheus e removendo
// a dependência externa gorilla/mux.
// BuildRouter creates the main HTTP router for the application.
//
// Go 1.22+ Enhanced Routing: net/http.ServeMux nativo suporta método + padrão
// de caminho (ex: "GET /api/v1/beers/{id}"). O roteador expõe r.Pattern com o
// template estático da rota, eliminando cardinalidade no Prometheus e removendo
// a dependência externa gorilla/mux.
func BuildRouter(db *sql.DB, logger *slog.Logger) http.Handler {
	return BuildRouterWithDBErr(db, nil, logger)
}

// BuildRouterWithDBErr é idêntico a BuildRouter, mas propaga o erro de
// inicialização da base de dados (se houver) para o controller de saúde, que o
// expõe em /health — essencial para diagnosticar falhas de ligação/SSL em
// ambientes serverless (ex: Vercel) sem acesso aos logs do processo.
func BuildRouterWithDBErr(db *sql.DB, dbErr error, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if dbErr != nil && db == nil {
		slog.Warn("base de dados indisponível no arranque; rotas de dados retornarão 503", "err", dbErr)
	}

	// Repositórios Postgres. Se o DB não estiver disponível (db nil ou Ping
	// falha), mantemos o servidor de pé para /docs, /health e /metrics; as
	// rotas de dados retornam 503 específico em vez de derrubar o processo.
	var beerRepo beerRepository.BeerRepository
	var err error
	beerRepo, err = beerRepository.NewPostgresBeerRepository(db)
	if err != nil {
		slog.Error("repositório de cervejas indisponível; rotas de dados retornarão 503", "err", err)
		beerRepo = beerRepository.NewUnavailableBeerRepository()
	}

	var userRepo userRepository.UserRepository
	userRepo, err = userRepository.NewPostgresUserRepository(db)
	if err != nil {
		slog.Error("repositório de utilizadores indisponível; rotas de dados retornarão 503", "err", err)
		userRepo = userRepository.NewUnavailableUserRepository()
	}

	// Hub SSE único (singleton por processo) para difusão em tempo real.
	eventHub := realtime.NewHub(64)

	beerUsecase := beerUsecase.NewBeerUsecase(beerRepo, eventHub)
	userUsecase := userUsecase.NewUserUsecase(userRepo)

	// Cliente de object storage para upload de mídia (RFC 7578). Opcional:
	// se não estiver configurado, o endpoint de upload devolve 501 e o
	// resto da API funciona normalmente.
	var uploader storage.Uploader
	if s, err := storage.NewSupabaseStorageFromEnv(); err != nil {
		slog.Warn("storage de mídia não configurado; upload desativado", "err", err)
	} else {
		uploader = s
	}

	// Seed de admin global (idempotente): cria/promove admin se ADMIN_EMAIL e
	// ADMIN_PASSWORD estiverem definidos. Não bloqueia o arranque se faltarem.
	if err := userUsecase.SeedAdmin(context.Background()); err != nil {
		slog.Error("falha no seed de admin", "err", err)
	}

	beerController := beerHttp.NewBeerController(beerUsecase, logger, uploader)
	userController := userHttp.NewUserController(userUsecase, logger)
	monitoringController := monitoring.NewMonitoringController(beerUsecase, userUsecase, logger, db, dbErr)

	// Go 1.22+ ServeMux: registro declarativo com método + padrão.
	mux := http.NewServeMux()

	// Middleware global aplicado via wrapping (RequestID + Metrics + Auth contextual).
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

	// BFF: payload único para a home do app móvel (sem N requests sequenciais).
	mux.HandleFunc("GET /api/v1/feed", beerController.GetHomeFeed)

	// SSE: stream de eventos em tempo real (substitui polling do cliente).
	mux.HandleFunc("GET /api/v1/stream", realtime.SSEHandler(eventHub))

	mux.HandleFunc("POST /api/v1/users/register", userController.Register)
	mux.HandleFunc("POST /api/v1/users/login", userController.Login)

	// Rotas protegidas por JWT.
	mux.HandleFunc("GET /api/v1/users/{id}", middleware.Auth(userController.GetProfile))
	mux.HandleFunc("PUT /api/v1/users/{id}", middleware.Auth(userController.UpdateProfile))
	mux.HandleFunc("DELETE /api/v1/users/{id}", middleware.Auth(userController.DeleteAccount))

	mux.HandleFunc("GET /api/v1/stats", monitoringController.GetStats)
	mux.HandleFunc("GET /api/v1/health", monitoringController.HealthCheck)

	mux.HandleFunc("GET /docs", docsHandler())
	mux.HandleFunc("GET /docs/openapi.yaml", openapiSpecHandler())
	if !appMetrics.IsServerlessRuntime() {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	// Encadeamento de middlewares: CORS -> Compression -> RequestID -> Metrics -> handler.
	var handler http.Handler = mux
	handler = middleware.CORSMiddleware(handler)
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.RequestIDMiddleware(handler)
	handler = middleware.CompressionMiddleware(handler)

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

	// Em runtime serverless o orçamento de cold-start é curto: não bloqueamos
	// com retries de 2s. Fazemos um único Ping com timeout de contexto — falha
	// rápido para o chamador aplicar o fallback (Pilar 4: defesa contra bloqueio).
	if isServerlessRuntime() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err = db.PingContext(ctx); err != nil {
			db.Close()
			slog.Error("falha no ping do banco de dados (serverless)", "err", err, "conn_string", maskPassword(dbConnString))
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
			slog.Error("banco de dados indisponível após várias tentativas", "conn_string", maskPassword(dbConnString))
			return nil, fmt.Errorf("banco de dados não está disponível após várias tentativas")
		}
	}

	if err := migrateDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	return db, nil
}

// isServerlessRuntime devolve true quando a aplicação corre em modo serverless
// (Vercel/Now/AWS Lambda). Usado para ajustar o orçamento de cold-start (ex:
// não bloquear com retries de Ping) e desativar /metrics.
func isServerlessRuntime() bool {
	return os.Getenv("VERCEL") != "" || os.Getenv("NOW_REGION") != "" || os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

// resolveDBConnString devolve a connection string do PostgreSQL, tolerando
// as várias nomenclaturas usadas no projeto (DB_CONN_STRING, DBConnString) e,
// em último caso, compõe-a a partir das variáveis individuais (DB_USER,
// DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME, DB_SSLMODE). Isto torna o arranque
// resiliente a .env ausente no deploy ou a hosts distintos (docker "postgres"
// vs local "localhost") — Defense-in-Depth (Pilar 4).
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

func forceSupabaseSSL(connString string) string {
	if strings.Contains(connString, "supabase.co") && !strings.Contains(connString, "sslmode=") {
		sep := "?"
		if strings.Contains(connString, "?") {
			sep = "&"
		}
		return connString + sep + "sslmode=require"
	}
	return connString
}

func resolveDBConnString() string {
	for _, key := range []string{"DB_CONN_STRING", "DBConnString", "POSTGRES_URL_NON_POOLING", "POSTGRES_URL"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
		if v := viper.GetString(key); v != "" {
			return v
		}
	}

	user := firstNonEmpty(os.Getenv("DB_USER"), viper.GetString("DB_USER"), os.Getenv("POSTGRES_USER"))
	pass := firstNonEmpty(os.Getenv("DB_PASSWORD"), viper.GetString("DB_PASSWORD"), os.Getenv("POSTGRES_PASSWORD"))
	host := firstNonEmpty(os.Getenv("DB_HOST"), viper.GetString("DB_HOST"), os.Getenv("POSTGRES_HOST"), "localhost")
	port := firstNonEmpty(os.Getenv("DB_PORT"), viper.GetString("DB_PORT"), "5432")
	name := firstNonEmpty(os.Getenv("DB_NAME"), viper.GetString("DB_NAME"), os.Getenv("POSTGRES_DATABASE"), "postgres")
	sslmode := firstNonEmpty(os.Getenv("DB_SSLMODE"), viper.GetString("DB_SSLMODE"), "require")

	if user == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(user), url.QueryEscape(pass), host, port, name, sslmode)
}

// firstNonEmpty devolve o primeiro valor não-vazio (helper zero-alloc de
// curta duração, vive na stack).
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
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
		"migrations/add_role_to_users.sql",
		"migrations/create_beers_table.sql",
		"migrations/add_created_by_to_beers.sql",
		"migrations/add_created_at_to_beers.sql",
		"migrations/add_comments_jsonb.sql",
		"migrations/create_indexes.sql",
		"migrations/use_uuid_pk.sql",
		"migrations/add_updated_at.sql",
		"migrations/extend_media.sql",
		"migrations/drop_legacy_comments_table.sql",
	}

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
// Se o banco não estiver disponível no arranque, mantemos o servidor de pé:
// BuildRouter aplica o fallback UnavailableBeerRepository, então /docs,
// /health e /metrics continuam operacionais e só as rotas de dados retornam
// 503 (em vez de um 503 global em tudo). Não bloqueamos com handler de erro.
func InitializeVercelHandler() http.Handler {
	logger := slog.Default()

	db, err := InitDBFromEnv()
	if err != nil {
		slog.Warn("vercel bootstrap warning; rotas de dados retornarão 503", "err", err)
	}

	return BuildRouterWithDBErr(db, err, logger)
}
