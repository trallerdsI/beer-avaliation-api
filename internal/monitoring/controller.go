package controller

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"beer-review-app/internal/beer/usecase"
	userCase "beer-review-app/internal/user/usecase"
	"beer-review-app/pkg/response"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Structs de resposta para tipagem forte e melhor documentação Swagger

type RecentActivity struct {
	Type      string    `json:"type"`
	UserID    string    `json:"userId"`
	BeerID    string    `json:"beerId"`
	Timestamp time.Time `json:"timestamp"` // O Go serializa automaticamente para RFC3339
}

type StatsResponse struct {
	TotalBeers     int64            `json:"totalBeers"`
	TopBeers       interface{}      `json:"topBeers"` // Substitua interface{} pela sua struct real de Beer se disponível
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
	beerUsecase    usecase.BeerUsecase
	userUsecase    userCase.UserUsecase // Mantido para compatibilidade, mas atualmente sem uso
	logger         *zap.Logger
	startTime      time.Time
	metricsHandler http.Handler // Cache do handler do Prometheus
}

func NewMonitoringController(bu usecase.BeerUsecase, uu userCase.UserUsecase, logger *zap.Logger) *MonitoringController {
	return &MonitoringController{
		beerUsecase:    bu,
		userUsecase:    uu,
		logger:         logger,
		startTime:      time.Now(),
		metricsHandler: promhttp.Handler(), // Inicializado uma única vez
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
		c.logger.Error("Failed to get beers for stats", zap.Error(err))
		response.SendError(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	stats := StatsResponse{
		TotalBeers:     totalBeers, // Corrigido de len(beers) para o total real retornado do usecase
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
	runtime.ReadMemStats(&m) // Nota: Esta chamada faz um STW muito curto, ideal para rotas não-críticas de monitoramento

	health := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  time.Since(c.startTime).Truncate(time.Second).String(), // Uptime limpo sem nanossegundos poluindo o JSON
		Memory: MemoryStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
		Dependencies: map[string]string{
			"database": c.checkDatabaseHealth(r.Context()), // Propagação correta de contexto
		},
	}

	response.SendResponse(w, http.StatusOK, health)
}

// @Summary Get application metrics
// @Description Get Prometheus metrics
// @Produce text/plain
// @Security BearerAuth
// @Success 200 {string} string
// @Router /metrics [get]
func (c *MonitoringController) Metrics(w http.ResponseWriter, r *http.Request) {
	// Reutiliza o handler cacheado na struct, evitando alocações por requisição
	c.metricsHandler.ServeHTTP(w, r)
}

func (c *MonitoringController) getRecentActivity(_ context.Context) []RecentActivity {
	// Exemplo de retorno tipado e limpo sem alocação dinâmica de chaves de string de forma genérica
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
	// Na implementação real, use o ctx recebido para respeitar timeouts
	// Exemplo: err := c.db.PingContext(ctx)
	return "up"
}