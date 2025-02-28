package monitoring

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

type MonitoringController struct {
	beerUsecase usecase.BeerUsecase
	userUsecase userCase.UserUsecase
	logger      *zap.Logger
	startTime   time.Time
}

func NewMonitoringController(bu usecase.BeerUsecase, uu userCase.UserUsecase, logger *zap.Logger) *MonitoringController {
	return &MonitoringController{
		beerUsecase: bu,
		userUsecase: uu,
		logger:      logger,
		startTime:   time.Now(),
	}
}

// @Summary Get application statistics
// @Description Get statistics about beers, users, and activity
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /stats [get]
func (c *MonitoringController) GetStats(w http.ResponseWriter, r *http.Request) {
	beers, _, err := c.beerUsecase.GetPaginated(r.Context(), 1, 5) // Get top 5 beers
	if err != nil {
		c.logger.Error("Failed to get beers for stats", zap.Error(err))
		response.SendError(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	stats := map[string]interface{}{
		"totalBeers":     len(beers),
		"topBeers":       beers,
		"recentActivity": c.getRecentActivity(r.Context()),
	}

	response.SendResponse(w, http.StatusOK, stats)
}

// @Summary Get application health status
// @Description Check the health of the application and its dependencies
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (c *MonitoringController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health := map[string]interface{}{
		"status":  "healthy",
		"version": "1.0.0",
		"uptime":  time.Since(c.startTime).String(),
		"memory": map[string]interface{}{
			"alloc":      m.Alloc,
			"totalAlloc": m.TotalAlloc,
			"sys":        m.Sys,
			"numGC":      m.NumGC,
		},
		"dependencies": map[string]string{
			"database": c.checkDatabaseHealth(),
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
	promhttp.Handler().ServeHTTP(w, r)
}

func (c *MonitoringController) getRecentActivity(_ context.Context) []map[string]interface{} {
	// This would typically come from a database query
	return []map[string]interface{}{
		{
			"type":      "comment",
			"userId":    "user123",
			"beerId":    "beer456",
			"timestamp": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		},
		{
			"type":      "like",
			"userId":    "user789",
			"beerId":    "beer456",
			"timestamp": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		},
	}
}

func (c *MonitoringController) checkDatabaseHealth() string {
	// Implement actual database health check
	return "up"
}

