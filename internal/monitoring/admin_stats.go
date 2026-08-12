package controller

import (
	"time"

	"beer-review-app/internal/beer/repository"
)

// AdminStatsResponse é o payload do endpoint /api/v1/admin/stats.
type AdminStatsResponse struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Users       AdminUsers      `json:"users"`
	Beers       AdminBeers      `json:"beers"`
	Engagement  AdminEngagement `json:"engagement"`
	System      AdminSystem     `json:"system"`
}

type AdminUsers struct {
	Total       int            `json:"total"`
	NewThisWeek int            `json:"new_this_week"`
	ByProvider  map[string]int `json:"by_provider"`
	AdminsCount int            `json:"admins_count"`
}

type AdminBeers struct {
	Total                  int                     `json:"total"`
	TopStyles              []repository.StyleCount `json:"top_styles"`
	AddedLast30Days        int                     `json:"added_last_30_days"`
	CommunityContributions CommunityBeerStats      `json:"community_contributions"`
}

type CommunityBeerStats struct {
	UserCreated   int `json:"user_created"`
	SystemCreated int `json:"system_created"`
}

type AdminEngagement struct {
	TotalComments     int                           `json:"total_comments"`
	Sentiment         repository.SentimentStats     `json:"sentiment"`
	TotalLikes        int                           `json:"total_likes"`
	MostCommentedBeer *repository.MostCommentedBeer `json:"most_commented_beer,omitempty"`
}

type AdminSystem struct {
	LastBeerCreatedAt *time.Time `json:"last_beer_created_at,omitempty"`
	LastDBUpdate      *time.Time `json:"last_db_update,omitempty"`
}
