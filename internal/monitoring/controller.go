package controller

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	beerRepo "beer-review-app/internal/beer/repository"
	"beer-review-app/internal/beer/usecase"
	userRepo "beer-review-app/internal/user/repository"
	"beer-review-app/pkg/errors"
	"beer-review-app/pkg/middleware"
	"beer-review-app/pkg/response"
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
	beerUsecase usecase.BeerUsecase
	beerRepo    beerRepo.BeerRepository
	userRepo    userRepo.UserRepository
	logger      *slog.Logger
	startTime   time.Time
	db          *sql.DB // opcional: nil em runtime offline/serverless desativa o ping de DB
	dbErr       error   // erro de inicialização da BD (ex: sem DB_CONN_STRING); exposto em /health
}

func NewMonitoringController(bu usecase.BeerUsecase, beerRepo beerRepo.BeerRepository, userRepo userRepo.UserRepository, logger *slog.Logger, db *sql.DB, dbErr error) *MonitoringController {
	if logger == nil {
		logger = slog.Default()
	}
	return &MonitoringController{
		beerUsecase: bu,
		beerRepo:    beerRepo,
		userRepo:    userRepo,
		logger:      logger,
		startTime:   time.Now(),
		db:          db,
		dbErr:       dbErr,
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
// @Description Check the health of the application and its dependencies
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (c *MonitoringController) HealthCheck(w http.ResponseWriter, r *http.Request) {
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
		Uptime:  time.Since(c.startTime).Truncate(time.Second).String(),
		Dependencies: map[string]string{
			"database": dbStatus,
		},
	}
	if dbDetail != "" {
		health.Dependencies["database_detail"] = dbDetail
	}

	response.SendResponse(w, status, health)
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
