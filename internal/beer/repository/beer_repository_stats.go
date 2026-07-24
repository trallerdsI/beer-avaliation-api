package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AdminStats agrega métricas globais para o painel administrativo.
type AdminStats struct {
	TotalUsers        int
	NewUsersWeek      int
	UsersByProvider   map[string]int
	AdminsCount       int
	TotalBeers        int
	TopStyles         []StyleCount
	AddedLast30Days   int
	UserCreatedBeers  int
	SystemCreatedBeers int
	TotalComments     int
	Sentiment         SentimentStats
	TotalLikes        int
	MostCommentedBeer *MostCommentedBeer
	LastBeerCreatedAt *time.Time
	LastDBUpdate      *time.Time
}

type StyleCount struct {
	Style string
	Count int
}

type SentimentStats struct {
	Positive         int     `json:"positive"`
	Negative         int     `json:"negative"`
	SatisfactionRate float64 `json:"satisfaction_rate"`
}

type MostCommentedBeer struct {
	ID       string
	Name     string
	Comments int
}

// UserStats agrega métricas pessoais do utilizador autenticado.
type UserStats struct {
	UserID         string
	MemberSince    time.Time
	BeersReviewed  int
	TotalComments  int
	LikesGiven     int
	LikesReceived  int
	PositiveRatio  float64
	FavoriteStyles []StyleCount
	TopAromaNotes  string
	BeersAdded     int
}

// GetAdminStats devolve o painel analítico completo para administradores.
// Toda a computação é feita no Postgres (COUNT, SUM, FILTER, GROUP BY),
// sem trazer linhas brutas para memória do Go.
func (r *PostgresBeerRepository) GetAdminStats(ctx context.Context) (*AdminStats, error) {
	var stats AdminStats
	stats.UsersByProvider = make(map[string]int)

	// 1. Total users
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beerUsers`).Scan(&stats.TotalUsers); err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// 2. New users last 7 days
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beerUsers WHERE created_at >= NOW() - INTERVAL '7 days'`).Scan(&stats.NewUsersWeek); err != nil {
		return nil, fmt.Errorf("failed to count new users: %w", err)
	}

	// 3. Users by provider
	rows, err := r.db.QueryContext(ctx, `
		SELECT provider, COUNT(*) FROM beerUsers GROUP BY provider`)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by provider: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}
		stats.UsersByProvider[provider] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating provider rows: %w", err)
	}

	// 4. Active admins
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beerUsers WHERE role = 'admin'`).Scan(&stats.AdminsCount); err != nil {
		return nil, fmt.Errorf("failed to count admins: %w", err)
	}

	// 5. Total beers
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beers`).Scan(&stats.TotalBeers); err != nil {
		return nil, fmt.Errorf("failed to count beers: %w", err)
	}

	// 6. Top styles (GROUP BY)
	rows, err = r.db.QueryContext(ctx, `
		SELECT style, COUNT(*) as cnt FROM beers GROUP BY style ORDER BY cnt DESC LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("failed to get top styles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc StyleCount
		if err := rows.Scan(&sc.Style, &sc.Count); err != nil {
			return nil, fmt.Errorf("failed to scan style: %w", err)
		}
		stats.TopStyles = append(stats.TopStyles, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating style rows: %w", err)
	}

	// 7. Added last 30 days
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beers WHERE created_at >= NOW() - INTERVAL '30 days'`).Scan(&stats.AddedLast30Days); err != nil {
		return nil, fmt.Errorf("failed to count recent beers: %w", err)
	}

	// 8. Community contributions (created_by != '' AND created_by IS NOT NULL)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beers WHERE created_by IS NOT NULL AND created_by != ''`).Scan(&stats.UserCreatedBeers); err != nil {
		return nil, fmt.Errorf("failed to count user created beers: %w", err)
	}
	stats.SystemCreatedBeers = stats.TotalBeers - stats.UserCreatedBeers

	// 9. Total comments (jsonb_array_length)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(jsonb_array_length(comments)), 0) FROM beers`).Scan(&stats.TotalComments); err != nil {
		return nil, fmt.Errorf("failed to count comments: %w", err)
	}

	// 10. Sentiment (positive vs negative from JSONB comments)
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM((elem->>'positive')::int), 0) FILTER (WHERE (elem->>'positive')::boolean = true),
			COALESCE(SUM((elem->>'positive')::int), 0) FILTER (WHERE (elem->>'positive')::boolean = false)
		FROM beers,
		jsonb_array_elements(comments) AS elem`).Scan(&stats.Sentiment.Positive, &stats.Sentiment.Negative); err != nil {
		return nil, fmt.Errorf("failed to get sentiment: %w", err)
	}
	if stats.TotalComments > 0 {
		stats.Sentiment.SatisfactionRate = float64(stats.Sentiment.Positive) / float64(stats.TotalComments) * 100
	}

	// 11. Total likes (sum of likes field in JSONB comments)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM((elem->>'likes')::int), 0)
		FROM beers,
		jsonb_array_elements(comments) AS elem`).Scan(&stats.TotalLikes); err != nil {
		return nil, fmt.Errorf("failed to count likes: %w", err)
	}

	// 12. Most commented beer
	if stats.TotalComments > 0 {
		var mc MostCommentedBeer
		if err := r.db.QueryRowContext(ctx, `
			SELECT id, name, jsonb_array_length(comments) as cnt
			FROM beers
			WHERE jsonb_array_length(comments) > 0
			ORDER BY cnt DESC
			LIMIT 1`).Scan(&mc.ID, &mc.Name, &mc.Comments); err != nil {
			if err != sql.ErrNoRows {
				return nil, fmt.Errorf("failed to get most commented beer: %w", err)
			}
		} else {
			stats.MostCommentedBeer = &mc
		}
	}

	// 13. Last beer created at
	if err := r.db.QueryRowContext(ctx, `
		SELECT created_at FROM beers ORDER BY created_at DESC LIMIT 1`).Scan(&stats.LastBeerCreatedAt); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get last beer created at: %w", err)
		}
	}

	// 14. Last DB update (most recent updated_at across tables)
	if err := r.db.QueryRowContext(ctx, `
		SELECT GREATEST(
			COALESCE((SELECT MAX(updated_at) FROM beers), 'epoch'),
			COALESCE((SELECT MAX(updated_at) FROM beerUsers), 'epoch')
		)`).Scan(&stats.LastDBUpdate); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get last db update: %w", err)
		}
	}

	return &stats, nil
}

// GetUserStats devolve métricas pessoais do utilizador autenticado.
// A computação é feita no Postgres, filtrando comments JSONB por user_id.
func (r *PostgresBeerRepository) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	var stats UserStats
	stats.UserID = userID

	// 1. Beers reviewed (comentários deste usuário)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM beers,
		jsonb_array_elements(comments) AS elem
		WHERE elem->>'createdBy' = $1`, userID).Scan(&stats.TotalComments); err != nil {
		return nil, fmt.Errorf("failed to count user comments: %w", err)
	}
	stats.BeersReviewed = stats.TotalComments

	// 2. Likes received (soma de likes nos comentários deste usuário)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM((elem->>'likes')::int), 0)
		FROM beers,
		jsonb_array_elements(comments) AS elem
		WHERE elem->>'createdBy' = $1`, userID).Scan(&stats.LikesReceived); err != nil {
		return nil, fmt.Errorf("failed to count likes received: %w", err)
	}

	// 3. Likes given (comentários onde o usuário deu like, via liked_by array)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM beers,
		jsonb_array_elements(comments) AS elem
		WHERE $1 = ANY(ARRAY(SELECT jsonb_array_elements_text(elem->'likedBy')))`, userID).Scan(&stats.LikesGiven); err != nil {
		return nil, fmt.Errorf("failed to count likes given: %w", err)
	}

	// 4. Positive ratio
	var positiveCount int
	if stats.TotalComments > 0 {
		if err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM beers,
			jsonb_array_elements(comments) AS elem
			WHERE elem->>'createdBy' = $1 AND (elem->>'positive')::boolean = true`, userID).Scan(&positiveCount); err != nil {
			return nil, fmt.Errorf("failed to count positive comments: %w", err)
		}
		stats.PositiveRatio = float64(positiveCount) / float64(stats.TotalComments) * 100
	}

	// 5. Favorite styles (top 5)
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.style, COUNT(*) as cnt
		FROM beers b
		JOIN jsonb_array_elements(b.comments) AS elem ON true
		WHERE elem->>'createdBy' = $1
		GROUP BY b.style
		ORDER BY cnt DESC
		LIMIT 5`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite styles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc StyleCount
		if err := rows.Scan(&sc.Style, &sc.Count); err != nil {
			return nil, fmt.Errorf("failed to scan style: %w", err)
		}
		stats.FavoriteStyles = append(stats.FavoriteStyles, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating style rows: %w", err)
	}

	// 6. Top aroma notes (mais frequente)
	if len(stats.FavoriteStyles) > 0 {
		var topAroma string
		if err := r.db.QueryRowContext(ctx, `
			SELECT aroma
			FROM beers b
			JOIN jsonb_array_elements(b.comments) AS elem ON true
			WHERE elem->>'createdBy' = $1
			GROUP BY aroma
			ORDER BY COUNT(*) DESC
			LIMIT 1`, userID).Scan(&topAroma); err != nil {
			if err != sql.ErrNoRows {
				return nil, fmt.Errorf("failed to get top aroma: %w", err)
			}
		} else {
			stats.TopAromaNotes = topAroma
		}
	}

	// 7. Beers added to catalog (created_by = user_id)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beers WHERE created_by = $1 AND created_by IS NOT NULL AND created_by != ''`, userID).Scan(&stats.BeersAdded); err != nil {
		return nil, fmt.Errorf("failed to count beers added: %w", err)
	}

	// 8. Member since (buscado no userRepository, mas delegado ao controller)
	// O controller preenche este campo a partir do userRepository.

	return &stats, nil
}
