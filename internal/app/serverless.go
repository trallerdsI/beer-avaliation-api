package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"beer-review-app/pkg/database"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"
)

// ResolveDSN expõe resolveDBConnString (wrapper idempotente). É seguro
// para uso no handler serverless antes do init completo do pool.
func ResolveDSN() string { return resolveDBConnString() }

// RouterHasDB devolve true se o handler foi construído com um pool não-nil.
// Em serverless, o handler começa com db=nil; após o primeiro TryInitDB
// bem-sucedido, o router é reconstruído com db=pool e este predicado
// passa a devolver true.
func RouterHasDB(h http.Handler) bool {
	if h == nil {
		return false
	}
	if t, ok := h.(*taggedRouter); ok {
		return t.hasDB.Load()
	}
	// Heurística: se o handler não foi marcado, assumimos que tem DB
	// (caso do cmd/server long-running).
	return true
}

// taggedRouter envolve o handler com um flag atômico que indica se o
// pool de DB foi resolvido. Permite ao wrapper serverless saber quando
// o hot-swap foi concluído.
type taggedRouter struct {
	http.Handler
	hasDB atomic.Bool
}

func wrapRouter(h http.Handler, hasDB bool) *taggedRouter {
	t := &taggedRouter{Handler: h}
	t.hasDB.Store(hasDB)
	return t
}

// TryInitDB inicializa o pool preguiçosamente. É a contraparte "sem
// migrations" de InitDB — em serverless, o schema já deve existir
// (criado pelo cmd/server eager ou pelo painel do provider). Falha
// rápida com timeout curto.
func TryInitDB(ctx context.Context, dsn string, logger *slog.Logger) (*database.RetryableDB, error) {
	if dsn == "" {
		return nil, errors.NewAppError(503, "database dsn is empty", nil)
	}
	if logger == nil {
		logger = slog.Default()
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, errors.NewAppError(503, "failed to open database", err)
	}
	db.SetMaxOpenConns(maxOpenConns())
	db.SetMaxIdleConns(maxIdleConns())
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		logger.Warn("lazy db ping failed", "err", err)
		return nil, errors.NewAppError(503, "database ping failed", err)
	}
	logger.Info("lazy db pool initialized on first request")
	return database.NewRetryableDB(db), nil
}

// WriteServiceUnavailable escreve um Problem Details 503 padronizado
// sem vazar a causa raiz (OWASP A05).
func WriteServiceUnavailable(w http.ResponseWriter, detail string) {
	if detail == "" {
		detail = "Recurso de persistência temporariamente indisponível."
	}
	response.SendProblem(w, errors.NewProblem(http.StatusServiceUnavailable, "service_unavailable", detail))
}

// NeedsDB decide se um path requer o pool de banco. Whitelist explícita
// do que é puramente lógico/estático: enums, health, docs, metrics,
// swagger UI. Tudo o mais passa pela camada de persistência.
func NeedsDB(path string) bool {
	static := []string{
		"/api/v1/beers/enums",
		"/api/v1/health",
		"/healthz",
		"/readyz",
		"/docs",
		"/docs/openapi.yaml",
		"/metrics",
	}
	for _, p := range static {
		if path == p {
			return false
		}
	}
	// Sufixos comumente estáticos (favicon, assets do swagger-ui).
	if strings.HasPrefix(path, "/docs/") || strings.HasPrefix(path, "/assets/") {
		return false
	}
	return true
}
