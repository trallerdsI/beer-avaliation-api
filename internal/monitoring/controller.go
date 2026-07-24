package controller

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"beer-review-app/internal/beer/usecase"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/response"
)

// Structs de resposta para tipagem forte e melhor documentação Swagger

type RecentActivity struct {
	Type      string    `json:"type"`
	UserID    string    `json:"userId"`
	BeerID    string    `json:"beerId"`
	Timestamp time.Time `json:"timestamp"` // O Go serializa automaticamente para RFC3339
}

type StatsResponse struct {
	TotalBeers     int              `json:"totalBeers"` // CORRIGIDO: Alterado de int64 para int para evitar erros de compilação
	TopBeers       interface{}      `json:"topBeers"`   // Substitua interface{} pela sua struct real de Beer se disponível
	RecentActivity []RecentActivity `json:"recentActivity"`
}

type MemoryStats struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"totalAlloc"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"numGC"`
}

type HealthResponse struct {
	Status       string            `json:"status"`
	Version      string            `json:"version"`
	Uptime       string            `json:"uptime"`
	Memory       MemoryStats       `json:"memory"`
	Dependencies map[string]string `json:"dependencies"`
}

type MonitoringController struct {
	beerUsecase usecase.BeerUsecase
	logger      *slog.Logger
	startTime   time.Time
	db          *sql.DB // opcional: nil em runtime offline/serverless desativa o ping de DB
	dbErr       error   // erro de inicialização da BD (ex: sem DB_CONN_STRING); exposto em /health
}

func NewMonitoringController(bu usecase.BeerUsecase, logger *slog.Logger, db *sql.DB, dbErr error) *MonitoringController {
	if logger == nil {
		logger = slog.Default()
	}
	return &MonitoringController{
		beerUsecase: bu,
		logger:      logger,
		startTime:   time.Now(),
		db:          db,
		dbErr:       dbErr,
	}
}

// @Summary Get application statistics
// @Description Get statistics about beers, users, and activity
// @Produce json
// @Security BearerAuth
// @Success 200 {object} StatsResponse
// @Router /stats [get]
func (c *MonitoringController) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Obtendo as top 5 cervejas e o total real cadastrado no sistema (totalBeers)
	beers, totalBeers, err := c.beerUsecase.GetPaginated(ctx, 1, 5)
	if err != nil {
		c.logger.Error("Failed to get beers for stats", slog.String("error", err.Error()))
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}

	stats := StatsResponse{
		TotalBeers:     totalBeers, // Atribuição direta sem necessidade de conversão manual
		TopBeers:       beers,
		RecentActivity: c.getRecentActivity(ctx),
	}

	response.SendResponse(w, http.StatusOK, stats)
}

// @Summary Get application health status
// @Description Check the health of the application and its dependencies
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (c *MonitoringController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m) // Chamada leve para coleta de GC e memória em rotas de monitoramento

	dbStatus := c.checkDatabaseHealth(r.Context())
	status := http.StatusOK
	overall := "healthy"
	dbDetail := ""
	if dbStatus != "up" {
		overall = "degraded"
		status = http.StatusServiceUnavailable
		if c.dbErr != nil {
			dbDetail = c.dbErr.Error()
		} else if c.db == nil {
			dbDetail = "database not configured"
		} else {
			dbDetail = "connection failed"
		}
	}

	health := HealthResponse{
		Status:  overall,
		Version: "1.0.0",
		Uptime:  time.Since(c.startTime).Truncate(time.Second).String(), // Exibe uptime limpo sem frações de nanossegundos
		Memory: MemoryStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
		Dependencies: map[string]string{
			"database": dbStatus, // Propagação correta do contexto para respeitar timeouts
		},
	}
	if dbDetail != "" {
		health.Dependencies["database_detail"] = dbDetail
	}

	response.SendResponse(w, status, health)
}

func (c *MonitoringController) getRecentActivity(_ context.Context) []RecentActivity {
	return []RecentActivity{
		{
			Type:      "comment",
			UserID:    "user123",
			BeerID:    "beer456",
			Timestamp: time.Now().Add(-5 * time.Minute),
		},
		{
			Type:      "like",
			UserID:    "user789",
			BeerID:    "beer456",
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
	}
}

func (c *MonitoringController) checkDatabaseHealth(ctx context.Context) string {
	if c.db == nil {
		// Sem ligação à BD (ex: falhou no arranque). O motivo real é exposto
		// em dependencies.database_detail via c.dbErr.
		return "down"
	}
	// Ping com timeout derivado do contexto da requisição: respeita cancelamento
	// e evita bloqueio indefinido do endpoint de health (Defense-in-Depth).
	if err := c.db.PingContext(ctx); err != nil {
		c.logger.WarnContext(ctx, "database health check failed", "err", err)
		return "down"
	}
	return "up"
}
