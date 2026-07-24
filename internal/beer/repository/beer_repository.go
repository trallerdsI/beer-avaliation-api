package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	_ "github.com/lib/pq"
)

// BeerRepository defines the interface for beer storage.
type BeerRepository interface {
	GetAll(context.Context) ([]model.Beer, error)
	Create(ctx context.Context, beer *model.Beer) error
	GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error)
	GetByID(ctx context.Context, id string) (model.Beer, error)
	Update(ctx context.Context, id string, beer model.Beer) error
	Delete(ctx context.Context, id string) error
	AddComment(ctx context.Context, id string, comment model.Comment) error
	DeleteComment(ctx context.Context, id string, commentID string) error
	SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error)
}

// PostgresBeerRepository is a PostgreSQL implementation of BeerRepository.
type PostgresBeerRepository struct {
	db *sql.DB
}

// NewPostgresBeerRepository creates a new PostgreSQL beer repository.
func NewPostgresBeerRepository(db *sql.DB) (*PostgresBeerRepository, error) {
	// Defesa (Pilar 4): db nil (init sem banco) devolve erro em vez de panicar
	// em db.Ping(); o BuildRouter aplica então o fallback UnavailableBeerRepository.
	if db == nil {
		return nil, errors.NewUnavailableError()
	}
	// Check if the connection is valid
	if err := db.Ping(); err != nil {
		return nil, errors.NewAppError(503, "beer database unavailable", err)
	}

	return &PostgresBeerRepository{db: db}, nil
}

// Create adds a new beer to the PostgreSQL database.
// O id é um UUIDv7 (RFC 9562) gerado pela aplicação e já preenchido em
// beer.ID antes da chamada (ver beerUsecase.Create). Isto garante IDs
// time-ordered não sequenciais e evita o round-trip RETURNING id.
func (r *PostgresBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	// Initialize Comments as an empty array if it's nil
	if beer.Comments == nil {
		beer.Comments = []model.Comment{}
	}

	commentsJSON, err := json.Marshal(beer.Comments)
	if err != nil {
		return fmt.Errorf("failed to marshal comments: %w", err)
	}

	// Inserting the beer into the beers table (id vindo do app como UUIDv7).
	// created_by regista o dono (AuthZ: só criador ou admin editam/apagam).
	// created_at é definido no usecase (UTC) para ordenação cronológica.
	_, err = r.db.ExecContext(ctx, `
   INSERT INTO beers (
      id, name, style, description, image_url, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by, created_at
   ) VALUES ($1, $2, COALESCE($3, NULL), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		beer.ID, beer.Name, beer.Style, beer.Description, beer.ImageUrl, beer.Alcohol, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Finish, commentsJSON, beer.CreatedBy, beer.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetByID retrieves a beer by its ID from the PostgreSQL database.
func (r *PostgresBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	var beer model.Beer
	var commentsJSON, mediaJSON []byte
	err := r.db.QueryRowContext(ctx, `
        SELECT
            id,
            name,
            style,
            description,
            image_url,
            alcohol,
            taste,
            aroma,
            color,
            body,
            carbonation,
            finish,
            comments,
            created_by,
            created_at,
            updated_at,
            media
        FROM beers WHERE id = $1`, id).
		Scan(
			&beer.ID,
			&beer.Name,
			&beer.Style,
			&beer.Description,
			&beer.ImageUrl,
			&beer.Alcohol,
			&beer.Taste,
			&beer.Aroma,
			&beer.Color,
			&beer.Body,
			&beer.Carbonation,
			&beer.Finish,
			&commentsJSON,
			&beer.CreatedBy,
			&beer.CreatedAt,
			&beer.UpdatedAt,
			&mediaJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Beer{}, errors.NewAppError(404, "beer not found", nil)
		}
		return model.Beer{}, err
	}

	beer.Comments = unmarshalComments(commentsJSON)
	beer.Media = unmarshalMedia(mediaJSON)
	setRatingAggregates(&beer)
	return beer, nil
}

// unmarshalComments decodifica o JSONB de comentários com fallback seguro.
func unmarshalComments(data []byte) []model.Comment {
	if len(data) == 0 {
		return []model.Comment{}
	}
	var comments []model.Comment
	if err := json.Unmarshal(data, &comments); err != nil {
		slog.Error("erro ao decodificar comentários", "err", err)
		return []model.Comment{}
	}
	return comments
}

// unmarshalMedia decodifica o JSONB de mídias com fallback seguro (nunca nil).
func unmarshalMedia(data []byte) []model.MediaItem {
	if len(data) == 0 {
		return []model.MediaItem{}
	}
	var media []model.MediaItem
	if err := json.Unmarshal(data, &media); err != nil {
		slog.Error("erro ao decodificar mídias", "err", err)
		return []model.MediaItem{}
	}
	return media
}

// setRatingAggregates calcula averageRating e totalReviews a partir dos
// ratings dos comentários (Decisão B). Nota média com 1 casa decimal; 0 se
// não houver comentários com rating.
func setRatingAggregates(b *model.Beer) {
	var sum int
	var count int
	for _, c := range b.Comments {
		if c.Rating >= 1 && c.Rating <= 5 {
			sum += c.Rating
			count++
		}
	}
	b.TotalReviews = count
	if count > 0 {
		b.AverageRating = math.Round(float64(sum)/float64(count)*10) / 10
	}
}

func (r *PostgresBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	offset := (page - 1) * pageSize

	var beers []model.Beer = make([]model.Beer, 0)

	// Consultando as cervejas
	rows, err := r.db.QueryContext(ctx, `
        SELECT 
            id, 
            name, 
            style, 
            description, 
            image_url, 
            alcohol, 
            taste, 
            aroma, 
            color, 
            body, 
            carbonation, 
            finish,
            comments,
            created_by,
            created_at,
            updated_at,
            media
        FROM beers LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		slog.Error("error querying beers", "err", err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var beer model.Beer
		var commentsJSON, mediaJSON []byte
		var description, imageURL, createdBy, createdAt, updatedAt sql.NullString
		if err := rows.Scan(
			&beer.ID,
			&beer.Name,
			&beer.Style,
			&description,
			&imageURL,
			&beer.Alcohol,
			&beer.Taste,
			&beer.Aroma,
			&beer.Color,
			&beer.Body,
			&beer.Carbonation,
			&beer.Finish,
			&commentsJSON,
			&createdBy,
			&createdAt,
			&updatedAt,
			&mediaJSON); err != nil {
			slog.Error("error scanning beer", "err", err)
			return nil, 0, err
		}
		beer.Description = description.String
		beer.ImageUrl = imageURL.String
		beer.CreatedBy = createdBy.String
		beer.CreatedAt = createdAt.String
		beer.UpdatedAt = updatedAt.String
		beer.Comments = unmarshalComments(commentsJSON)
		setRatingAggregates(&beer)
		beers = append(beers, beer)
	}

	// Verificando erros após iterar sobre as linhas
	if err = rows.Err(); err != nil {
		slog.Error("error iterating over rows", "err", err)
		return nil, 0, err
	}

	// Obtendo a contagem total de cervejas
	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beers").Scan(&total)
	if err != nil {
		slog.Error("error getting total beer count", "err", err)
		return nil, 0, err
	}

	return beers, total, nil
}

// Update updates a beer in the PostgreSQL database.
func (r *PostgresBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	if beer.Comments == nil {
		beer.Comments = []model.Comment{}
	}
	if beer.Media == nil {
		beer.Media = []model.MediaItem{}
	}
	commentsJSON, err := json.Marshal(beer.Comments)
	if err != nil {
		return fmt.Errorf("failed to marshal comments: %w", err)
	}
	mediaJSON, err := json.Marshal(beer.Media)
	if err != nil {
		return fmt.Errorf("failed to marshal media: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		"UPDATE beers SET name=$1, style=$2, taste=$3, aroma=$4, color=$5, body=$6, carbonation=$7, alcohol=$8, finish=$9, comments=$10, media=$11, image_url=COALESCE(NULLIF($12, ''), image_url) WHERE id=$13",
		beer.Name, beer.Style, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Alcohol, beer.Finish, commentsJSON, mediaJSON, beer.ImageUrl, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.NewAppError(404, "beer not found", nil)
	}
	return nil
}

// Delete removes a beer from the PostgreSQL database.
func (r *PostgresBeerRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM beers WHERE id=$1", id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.NewAppError(404, "beer not found", nil)
	}
	return nil
}

func (r *PostgresBeerRepository) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	// strings.Builder com pré-alocação: evita as múltiplas realocações de
	// string causadas por concatenação "+=" e fmt.Sprintf no hot path (Pilar 2).
	var qb, cb strings.Builder
	qb.Grow(256)
	cb.Grow(128)
	qb.WriteString("SELECT id, name, style, description, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by, created_at, updated_at, media FROM beers WHERE 1=1")
	cb.WriteString("SELECT COUNT(*) FROM beers WHERE 1=1")
	args := []interface{}{}
	argPosition := 1

	// Add filters dynamically to query (parâmetros posicionais $N, sem concat
	// de valores — defesa contra SQL injection, Pilar 4).
	if filters.Query != "" {
		qb.WriteString(fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argPosition, argPosition))
		cb.WriteString(fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argPosition, argPosition))
		args = append(args, "%"+filters.Query+"%")
		argPosition++
	}

	if filters.Style != "" {
		qb.WriteString(fmt.Sprintf(" AND style = $%d", argPosition))
		cb.WriteString(fmt.Sprintf(" AND style = $%d", argPosition))
		args = append(args, filters.Style)
		argPosition++
	}

	if filters.MinAlcohol != nil {
		qb.WriteString(fmt.Sprintf(" AND alcohol >= $%d", argPosition))
		cb.WriteString(fmt.Sprintf(" AND alcohol >= $%d", argPosition))
		args = append(args, *filters.MinAlcohol)
		argPosition++
	}

	if filters.MaxAlcohol != nil {
		qb.WriteString(fmt.Sprintf(" AND alcohol <= $%d", argPosition))
		cb.WriteString(fmt.Sprintf(" AND alcohol <= $%d", argPosition))
		args = append(args, *filters.MaxAlcohol)
		argPosition++
	}

	if filters.Taste != "" {
		qb.WriteString(fmt.Sprintf(" AND taste = $%d", argPosition))
		cb.WriteString(fmt.Sprintf(" AND taste = $%d", argPosition))
		args = append(args, filters.Taste)
		argPosition++
	}

	// Get total count of filtered beers
	var total int
	err := r.db.QueryRowContext(ctx, cb.String(), args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count beers: %w", err)
	}

	// Add pagination
	qb.WriteString(fmt.Sprintf(" ORDER BY name LIMIT $%d OFFSET $%d", argPosition, argPosition+1))
	args = append(args, filters.PageSize, (filters.Page-1)*filters.PageSize)

	// Execute the query
	rows, err := r.db.QueryContext(ctx, qb.String(), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search beers: %w", err)
	}
	defer rows.Close()

	var beers []model.Beer = make([]model.Beer, 0)
	for rows.Next() {
		var beer model.Beer
		var commentsJSON, mediaJSON []byte
		var createdBy, createdAt, updatedAt sql.NullString
		err := rows.Scan(
			&beer.ID,
			&beer.Name,
			&beer.Style,
			&beer.Description,
			&beer.Alcohol,
			&beer.Taste,
			&beer.Aroma,
			&beer.Color,
			&beer.Body,
			&beer.Carbonation,
			&beer.Finish,
			&commentsJSON,
			&createdBy,
			&createdAt,
			&updatedAt,
			&mediaJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan beer: %w", err)
		}
		beer.CreatedBy = createdBy.String
		beer.CreatedAt = createdAt.String
		beer.UpdatedAt = updatedAt.String
		beer.Comments = unmarshalComments(commentsJSON)
		beer.Media = unmarshalMedia(mediaJSON)
		setRatingAggregates(&beer)
		beers = append(beers, beer)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over rows: %w", err)
	}

	return beers, total, nil
}

// GetAll retrieves all beers from the PostgreSQL database.
func (r *PostgresBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, style, description, image_url, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by, created_at, updated_at, media
		FROM beers
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get beers: %w", err)
	}
	defer rows.Close()

	var beers []model.Beer = make([]model.Beer, 0)
	for rows.Next() {
		var beer model.Beer
		var commentsJSON, mediaJSON []byte
		var description, imageURL, createdBy, createdAt, updatedAt sql.NullString
		err := rows.Scan(
			&beer.ID,
			&beer.Name,
			&beer.Style,
			&description,
			&imageURL,
			&beer.Alcohol,
			&beer.Taste,
			&beer.Aroma,
			&beer.Color,
			&beer.Body,
			&beer.Carbonation,
			&beer.Finish,
			&commentsJSON,
			&createdBy,
			&createdAt,
			&updatedAt,
			&mediaJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan beer: %w", err)
		}
		beer.Description = description.String
		beer.ImageUrl = imageURL.String
		beer.CreatedBy = createdBy.String
		beer.CreatedAt = createdAt.String
		beer.UpdatedAt = updatedAt.String
		beer.Comments = unmarshalComments(commentsJSON)
		beer.Media = unmarshalMedia(mediaJSON)
		setRatingAggregates(&beer)
		beers = append(beers, beer)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return beers, nil
}

// AddComment adds a comment to a beer in the PostgreSQL database.
func (r *PostgresBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	// First get the beer to ensure it exists
	beer, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get beer: %w", err)
	}

	// Add comment to the comments array
	beer.Comments = append(beer.Comments, comment)

	// Update the beer with the new comment
	return r.Update(ctx, id, beer)
}

// DeleteComment removes a comment from a beer in the PostgreSQL database.
func (r *PostgresBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	// First get the beer to ensure it exists
	beer, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get beer: %w", err)
	}

	// Find and remove the comment
	found := false
	for i, c := range beer.Comments {
		if c.ID == commentID {
			beer.Comments = append(beer.Comments[:i], beer.Comments[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("comment not found")
	}

	// Update the beer without the deleted comment
	return r.Update(ctx, id, beer)
}
