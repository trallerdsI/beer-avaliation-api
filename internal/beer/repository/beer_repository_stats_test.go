package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetAdminStats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		wantStats *AdminStats
		wantErr   bool
	}{
		{
			name: "success",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beerUsers$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created >= NOW").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT provider, COUNT\\(\\*\\).*GROUP BY provider").WillReturnRows(sqlmock.NewRows([]string{"provider", "count"}).AddRow("google", 80).AddRow("github", 20))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE role = 'admin'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beers$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))
				mock.ExpectQuery("SELECT style, COUNT\\(\\*\\).*GROUP BY style").WillReturnRows(sqlmock.NewRows([]string{"style", "cnt"}).AddRow("IPA", 20).AddRow("Stout", 15))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_at >= NOW").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_by IS NOT NULL").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(jsonb_array_length").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(200))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"positive", "negative"}).AddRow(150, 50))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(500))
				mock.ExpectQuery("SELECT id, name, jsonb_array_length\\(comments\\)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "cnt"}).AddRow("beer-1", "IPA", 50))
				mock.ExpectQuery("SELECT created_at FROM beers ORDER BY created_at DESC LIMIT 1").WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
				mock.ExpectQuery("SELECT GREATEST").WillReturnRows(sqlmock.NewRows([]string{"greatest"}).AddRow(time.Now()))
			},
			wantStats: &AdminStats{
				TotalUsers: 100, NewUsersWeek: 10, UsersByProvider: map[string]int{"google": 80, "github": 20},
				AdminsCount: 2, TotalBeers: 50, TopStyles: []StyleCount{{Style: "IPA", Count: 20}, {Style: "Stout", Count: 15}},
				AddedLast30Days: 5, UserCreatedBeers: 10, SystemCreatedBeers: 40,
				TotalComments: 200, Sentiment: SentimentStats{Positive: 150, Negative: 50, SatisfactionRate: 75.0},
				TotalLikes: 500, MostCommentedBeer: &MostCommentedBeer{ID: "beer-1", Name: "IPA", Comments: 50},
			},
		},
		{
			name: "error counting users",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beerUsers$").WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "error getting users by provider",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beerUsers$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created >= NOW").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT provider, COUNT\\(\\*\\).*GROUP BY provider").WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "no most commented beer",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beerUsers$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created >= NOW").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT provider, COUNT\\(\\*\\).*GROUP BY provider").WillReturnRows(sqlmock.NewRows([]string{"provider", "count"}))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE role = 'admin'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM beers$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT style, COUNT\\(\\*\\).*GROUP BY style").WillReturnRows(sqlmock.NewRows([]string{"style", "cnt"}))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_at >= NOW").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_by IS NOT NULL").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(jsonb_array_length").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"positive", "negative"}).AddRow(0, 0))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0))
				mock.ExpectQuery("SELECT created_at FROM beers ORDER BY created_at DESC LIMIT 1").WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery("SELECT GREATEST").WillReturnRows(sqlmock.NewRows([]string{"greatest"}).AddRow(time.Now()))
			},
			wantStats: &AdminStats{
				TotalUsers: 100, NewUsersWeek: 10, UsersByProvider: map[string]int{},
				AdminsCount: 0, TotalBeers: 0, TopStyles: []StyleCount{},
				AddedLast30Days: 0, UserCreatedBeers: 0, SystemCreatedBeers: 0,
				TotalComments: 0, Sentiment: SentimentStats{Positive: 0, Negative: 0, SatisfactionRate: 0},
				TotalLikes: 0, MostCommentedBeer: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer mockDB.Close()

			repo := &PostgresBeerRepository{db: mockDB}
			tt.setupMock(mock)

			got, err := repo.GetAdminStats(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetAdminStats() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.TotalUsers != tt.wantStats.TotalUsers {
				t.Errorf("TotalUsers = %v, want %v", got.TotalUsers, tt.wantStats.TotalUsers)
			}
			if got.NewUsersWeek != tt.wantStats.NewUsersWeek {
				t.Errorf("NewUsersWeek = %v, want %v", got.NewUsersWeek, tt.wantStats.NewUsersWeek)
			}
			if len(got.UsersByProvider) != len(tt.wantStats.UsersByProvider) {
				t.Errorf("UsersByProvider len = %v, want %v", len(got.UsersByProvider), len(tt.wantStats.UsersByProvider))
			}
			if got.TotalBeers != tt.wantStats.TotalBeers {
				t.Errorf("TotalBeers = %v, want %v", got.TotalBeers, tt.wantStats.TotalBeers)
			}
			if len(got.TopStyles) != len(tt.wantStats.TopStyles) {
				t.Errorf("TopStyles len = %v, want %v", len(got.TopStyles), len(tt.wantStats.TopStyles))
			}
			if got.TotalComments != tt.wantStats.TotalComments {
				t.Errorf("TotalComments = %v, want %v", got.TotalComments, tt.wantStats.TotalComments)
			}
			if got.Sentiment.Positive != tt.wantStats.Sentiment.Positive {
				t.Errorf("Sentiment.Positive = %v, want %v", got.Sentiment.Positive, tt.wantStats.Sentiment.Positive)
			}
			if got.TotalLikes != tt.wantStats.TotalLikes {
				t.Errorf("TotalLikes = %v, want %v", got.TotalLikes, tt.wantStats.TotalLikes)
			}
			if (got.MostCommentedBeer == nil) != (tt.wantStats.MostCommentedBeer == nil) {
				t.Errorf("MostCommentedBeer nil mismatch")
			} else if got.MostCommentedBeer != nil && tt.wantStats.MostCommentedBeer != nil {
				if got.MostCommentedBeer.ID != tt.wantStats.MostCommentedBeer.ID {
					t.Errorf("MostCommentedBeer.ID = %v, want %v", got.MostCommentedBeer.ID, tt.wantStats.MostCommentedBeer.ID)
				}
			}
		})
	}
}

func TestGetUserStats(t *testing.T) {
	ctx := context.Background()
	userID := "user-123"

	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		wantStats *UserStats
		wantErr   bool
	}{
		{
			name: "success with stats",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' =").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(25))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE \\$1 = ANY").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' = \\$1 AND \\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))
				mock.ExpectQuery("SELECT b.style, COUNT\\(\\*\\).*GROUP BY b.style").WillReturnRows(sqlmock.NewRows([]string{"style", "cnt"}).AddRow("IPA", 3).AddRow("Stout", 2))
				mock.ExpectQuery("SELECT aroma FROM beers b JOIN").WillReturnRows(sqlmock.NewRows([]string{"aroma"}).AddRow("fruity"))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_by =").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
			},
			wantStats: &UserStats{
				UserID: userID, BeersReviewed: 10, TotalComments: 10, LikesReceived: 25,
				LikesGiven: 5, PositiveRatio: 80.0, FavoriteStyles: []StyleCount{{Style: "IPA", Count: 3}, {Style: "Stout", Count: 2}},
				TopAromaNotes: "fruity", BeersAdded: 3,
			},
		},
		{
			name: "error counting user comments",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' =").WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "error getting favorite styles",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' =").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(25))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE \\$1 = ANY").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' = \\$1 AND \\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))
				mock.ExpectQuery("SELECT b.style, COUNT\\(\\*\\).*GROUP BY b.style").WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "no favorite styles skips top aroma",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE elem->>'createdBy' =").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT COALESCE\\(SUM\\(\\(elem->>").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE \\$1 = ANY").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT b.style, COUNT\\(\\*\\).*GROUP BY b.style").WillReturnRows(sqlmock.NewRows([]string{"style", "cnt"}))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\).*WHERE created_by =").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			wantStats: &UserStats{
				UserID: userID, BeersReviewed: 0, TotalComments: 0, LikesReceived: 0,
				LikesGiven: 0, PositiveRatio: 0, FavoriteStyles: []StyleCount{},
				TopAromaNotes: "", BeersAdded: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer mockDB.Close()

			repo := &PostgresBeerRepository{db: mockDB}
			tt.setupMock(mock)

			got, err := repo.GetUserStats(ctx, userID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetUserStats() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.UserID != tt.wantStats.UserID {
				t.Errorf("UserID = %v, want %v", got.UserID, tt.wantStats.UserID)
			}
			if got.BeersReviewed != tt.wantStats.BeersReviewed {
				t.Errorf("BeersReviewed = %v, want %v", got.BeersReviewed, tt.wantStats.BeersReviewed)
			}
			if got.LikesReceived != tt.wantStats.LikesReceived {
				t.Errorf("LikesReceived = %v, want %v", got.LikesReceived, tt.wantStats.LikesReceived)
			}
			if got.LikesGiven != tt.wantStats.LikesGiven {
				t.Errorf("LikesGiven = %v, want %v", got.LikesGiven, tt.wantStats.LikesGiven)
			}
			if got.BeersAdded != tt.wantStats.BeersAdded {
				t.Errorf("BeersAdded = %v, want %v", got.BeersAdded, tt.wantStats.BeersAdded)
			}
			if got.TopAromaNotes != tt.wantStats.TopAromaNotes {
				t.Errorf("TopAromaNotes = %v, want %v", got.TopAromaNotes, tt.wantStats.TopAromaNotes)
			}
		})
	}
}
