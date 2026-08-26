package controller

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	beerRepo "beer-review-app/internal/beer/repository"
	userRepo "beer-review-app/internal/user/repository"
	"beer-review-app/pkg/database"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"

	"github.com/redis/go-redis/v9"
)

// Structs de resposta para tipagem forte e melhor documentação Swagger

type StatsResponse struct {
	TotalBeers int                   `json:"totalBeers"`
	TopStyles  []beerRepo.StyleCount `json:"topStyles"`
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
	Dependencies map[string]string `json:"dependencies"`
}

type MonitoringController struct {
	beerRepo    beerRepo.BeerRepository
	userRepo    userRepo.UserRepository
	logger      *slog.Logger
	startTime   time.Time
	db          *database.RetryableDB
	dbErr       error
	redisClient *redis.Client
}

func NewMonitoringController(beerRepo beerRepo.BeerRepository, userRepo userRepo.UserRepository, logger *slog.Logger, db *database.RetryableDB, dbErr error, redisClient *redis.Client) *MonitoringController {
	if logger == nil {
		logger = slog.Default()
	}
	return &MonitoringController{
		beerRepo:    beerRepo,
		userRepo:    userRepo,
		logger:      logger,
		startTime:   time.Now(),
		db:          db,
		dbErr:       dbErr,
		redisClient: redisClient,
	}
}

// @Summary Get application statistics
// @Description Get public statistics about beers, users, and activity
// @Produce json
// @Success 200 {object} StatsResponse
// @Router /stats [get]
func (c *MonitoringController) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if c.beerRepo == nil {
		response.SendProblem(w, errors.NewProblem(http.StatusServiceUnavailable, "service_unavailable", "Estatísticas indisponíveis."))
		return
	}

	stats, err := c.beerRepo.GetAdminStats(ctx)
	if err != nil {
		c.logger.Error("Failed to get stats", slog.String("error", err.Error()))
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}

	publicStats := StatsResponse{
		TotalBeers: stats.TotalBeers,
		TopStyles:  stats.TopStyles,
	}

	response.SendResponse(w, http.StatusOK, publicStats)
}

// @Summary Get application health status
// @Description Returns 200 OK if the process is alive (liveness probe)
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (c *MonitoringController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.SendResponse(w, http.StatusOK, HealthResponse{
		Status: "alive",
	})
}

// @Summary Get admin statistics
// @Description Get full admin statistics panel (requires admin role)
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AdminStatsResponse
// @Router /admin/stats [get]
func (c *MonitoringController) GetAdminStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !middleware.IsAdmin(ctx) {
		response.SendProblem(w, errors.NewProblem(http.StatusForbidden, "forbidden", "É necessário privilégio de administrador."))
		return
	}

	if c.beerRepo == nil || c.userRepo == nil {
		response.SendProblem(w, errors.NewProblem(http.StatusServiceUnavailable, "service_unavailable", "Estatísticas indisponíveis."))
		return
	}

	stats, err := c.beerRepo.GetAdminStats(ctx)
	if err != nil {
		c.logger.Error("Failed to get admin stats", slog.String("error", err.Error()))
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}

	resp := AdminStatsResponse{
		GeneratedAt: time.Now().UTC(),
		Users: AdminUsers{
			Total:       stats.TotalUsers,
			NewThisWeek: stats.NewUsersWeek,
			ByProvider:  stats.UsersByProvider,
			AdminsCount: stats.AdminsCount,
		},
		Beers: AdminBeers{
			Total:           stats.TotalBeers,
			TopStyles:       stats.TopStyles,
			AddedLast30Days: stats.AddedLast30Days,
			CommunityContributions: CommunityBeerStats{
				UserCreated:   stats.UserCreatedBeers,
				SystemCreated: stats.SystemCreatedBeers,
			},
		},
		Engagement: AdminEngagement{
			TotalComments:     stats.TotalComments,
			Sentiment:         stats.Sentiment,
			TotalLikes:        stats.TotalLikes,
			MostCommentedBeer: stats.MostCommentedBeer,
		},
		System: AdminSystem{
			LastBeerCreatedAt: stats.LastBeerCreatedAt,
			LastDBUpdate:      stats.LastDBUpdate,
		},
	}

	response.SendResponse(w, http.StatusOK, resp)
}

// @Summary Get user statistics
// @Description Get authenticated user personal statistics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserStatsResponse
// @Router /users/me/stats [get]
func (c *MonitoringController) GetUserStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		response.SendProblem(w, errors.NewProblem(http.StatusUnauthorized, "unauthorized", "Utilizador não autenticado."))
		return
	}

	if c.beerRepo == nil || c.userRepo == nil {
		response.SendProblem(w, errors.NewProblem(http.StatusServiceUnavailable, "service_unavailable", "Estatísticas indisponíveis."))
		return
	}

	userStats, err := c.beerRepo.GetUserStats(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to get user stats", slog.String("error", err.Error()), "user_id", userID)
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}
	if userStats == nil {
		c.logger.Error("Failed to get user stats: nil result", "user_id", userID)
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}

	memberSince, err := c.userRepo.GetMemberSince(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to get member since", slog.String("error", err.Error()), "user_id", userID)
		response.SendProblem(w, errors.NewProblem(http.StatusInternalServerError, "internal_server_error", "Falha ao obter estatísticas."))
		return
	}

	resp := UserStatsResponse{
		UserID:      userID,
		MemberSince: memberSince,
		Activity: UserActivity{
			BeersReviewed: userStats.BeersReviewed,
			TotalComments: userStats.TotalComments,
			LikesGiven:    userStats.LikesGiven,
			LikesReceived: userStats.LikesReceived,
		},
		Preferences: UserPreferences{
			PositiveRatingRatio: userStats.PositiveRatio,
			FavoriteStyles:      userStats.FavoriteStyles,
			TopAromaNotes:       userStats.TopAromaNotes,
		},
		Contributions: UserContributions{
			BeersAddedToCatalog: userStats.BeersAdded,
		},
	}

	response.SendResponse(w, http.StatusOK, resp)
}

// @Summary Liveness probe
// @Description Returns 200 OK if the process is alive
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /healthz [get]
func (c *MonitoringController) LivenessProbe(w http.ResponseWriter, r *http.Request) {
	response.SendResponse(w, http.StatusOK, HealthResponse{
		Status: "alive",
	})
}

// @Summary Readiness probe
// @Description Returns 200 OK if the app is ready to receive traffic (DB reachable)
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} errors.Problem
// @Router /readyz [get]
func (c *MonitoringController) ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	dbStatus := c.checkDatabaseHealth(ctx)
	redisStatus := c.checkRedisHealth(ctx)
	if dbStatus != "up" || redisStatus != "up" {
		response.SendResponse(w, http.StatusServiceUnavailable, HealthResponse{
			Status:       "not_ready",
			Dependencies: map[string]string{"database": dbStatus, "redis": redisStatus},
		})
		return
	}

	response.SendResponse(w, http.StatusOK, HealthResponse{
		Status:       "ready",
		Dependencies: map[string]string{"database": "up", "redis": "up"},
	})
}

func (c *MonitoringController) checkDatabaseHealth(ctx context.Context) string {
	if c.db == nil {
		return "down"
	}
	if err := c.db.PingContext(ctx); err != nil {
		c.logger.WarnContext(ctx, "database health check failed", "err", err)
		return "down"
	}
	return "up"
}

func (c *MonitoringController) checkRedisHealth(ctx context.Context) string {
	if c.redisClient == nil {
		return "not_configured"
	}
	if err := c.redisClient.Ping(ctx).Err(); err != nil {
		c.logger.WarnContext(ctx, "redis health check failed", "err", err)
		return "down"
	}
	return "up"
}
