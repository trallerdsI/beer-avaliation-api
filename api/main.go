package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"beer-review-app/internal/app"
	"beer-review-app/pkg/logging"
)

// routerSlot segura o http.Handler ativo para a instância serverless.
// Em cold start, aponta para um router "degraded" (db=nil) — rotas
// estáticas como /docs, /api/v1/beers/enums, /healthz respondem
// imediatamente, sem aguardar o ping do Postgres. Rotas de dados
// devolvem 503 com Problem Details (RFC 7807).
//
// Após o primeiro request bem-sucedido de DB, o slot é substituído por
// um router completo (db=pool). A troca é atômica — readers não veem
// estado intermediário.
type routerSlot struct {
	current atomic.Pointer[http.Handler]
	log     *slog.Logger
}

var router *routerSlot

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, logging.SanitizeOptions(&slog.HandlerOptions{Level: slog.LevelInfo})))
	router = &routerSlot{log: logger}

	// Cold start otimista: monta o router com db=nil para que rotas
	// puramente lógicas respondam sem aguardar o ping do Postgres.
	// O flag hasDB=false sinaliza ao Handler que precisa inicializar
	// o pool preguiçosamente antes de rotas de dados.
	degraded := app.BuildRouter(nil, logger)
	router.current.Store(&degraded)
}

// Handler é o entrypoint serverless (Vercel). Estratégia:
//  1. Fast path: warm container com pool já inicializado — atende
//     imediatamente sem novo ping.
//  2. Slow path: cold start OU primeira chamada de DB OU pool caiu.
//     Tenta inicializar preguiçosamente (timeout curto) e substitui
//     o router de forma atômica para o request atual e futuros.
//  3. Se o ping falhar e a rota precisa de DB, devolve 503 com
//     Problem Details. Rotas estáticas continuam funcionando.
func Handler(w http.ResponseWriter, r *http.Request) {
	current := *router.current.Load()

	// Fast path: warm container com pool OK.
	if app.RouterHasDB(current) {
		current.ServeHTTP(w, r)
		return
	}

	// Slow path: cold start, primeira chamada de DB, ou pool caiu.
	// Rotas estáticas podem passar sem ping — não penalizamos o
	// cliente por algo que não usa.
	if !app.NeedsDB(r.URL.Path) {
		current.ServeHTTP(w, r)
		return
	}

	dsn := app.ResolveDSN()
	if dsn == "" {
		app.WriteServiceUnavailable(w, "database connection string is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	pool, err := app.TryInitDB(ctx, dsn, router.log)
	if err != nil {
		app.WriteServiceUnavailable(w, "database unavailable on cold start")
		return
	}

	// Hot-swap atômico: novos requests pulam o TryInitDB.
	full := app.BuildRouter(pool, router.log)
	router.current.Store(&full)

	// Serve a request atual com o router completo.
	(*router.current.Load()).ServeHTTP(w, r)
}
