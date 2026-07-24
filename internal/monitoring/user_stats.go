package controller

import (
	"time"

	"beer-review-app/internal/beer/repository"
)

// UserStatsResponse é o payload do endpoint /api/v1/users/me/stats.
type UserStatsResponse struct {
	UserID      string          `json:"user_id"`
	MemberSince time.Time       `json:"member_since"`
	Activity    UserActivity    `json:"activity"`
	Preferences UserPreferences `json:"preferences"`
	Contributions UserContributions `json:"contributions"`
}

type UserActivity struct {
	BeersReviewed int `json:"beers_reviewed"`
	TotalComments int `json:"total_comments"`
	LikesGiven    int `json:"likes_given"`
	LikesReceived int `json:"likes_received"`
}

type UserPreferences struct {
	PositiveRatingRatio float64             `json:"positive_rating_ratio"`
	FavoriteStyles      []repository.StyleCount `json:"favorite_styles"`
	TopAromaNotes       string              `json:"top_aroma_notes,omitempty"`
}

type UserContributions struct {
	BeersAddedToCatalog int `json:"beers_added_to_catalog"`
}
