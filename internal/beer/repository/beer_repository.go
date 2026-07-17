package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	_ "github.com/lib/pq"
)

// BeerRepository defines the interface for beer storage.
// In your repository package (repository/beer_repository.go)
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

// InMemoryBeerRepository is an in-memory implementation of BeerRepository.
type InMemoryBeerRepository struct {
	beers []model.Beer
	mutex sync.RWMutex
}

// NewInMemoryBeerRepository creates a new in-memory beer repository.
func NewInMemoryBeerRepository() *InMemoryBeerRepository {
	return &InMemoryBeerRepository{beers: []model.Beer{}}
}

// Create adds a new beer to the in-memory repository.
func (r *InMemoryBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.beers = append(r.beers, *beer)
	return nil
}

// GetByID retrieves a beer by its ID from the in-memory repository.
func (r *InMemoryBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, beer := range r.beers {
		if beer.ID == id {
			return beer, nil
		}
	}
	return model.Beer{}, errors.NewAppError(404, "beer not found", nil)
}

// GetAll retrieves all beers from the in-memory repository.
// Devolve uma CÓPIA defensiva: o caller não pode mutar o slice interno nem
// causar race concorrente (Pilar 4 / Pilar 3).
func (r *InMemoryBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	out := make([]model.Beer, len(r.beers))
	copy(out, r.beers)
	return out, nil
}

// GetPaginated retrieves paginated beers from the in-memory repository
func (r *InMemoryBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	start := (page - 1) * pageSize
	total := len(r.beers)

	if start >= total || pageSize <= 0 {
		return []model.Beer{}, total, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	// Cópia do segmento: evita expor o slice interno (sub-slice partilhada).
	seg := r.beers[start:end]
	out := make([]model.Beer, len(seg))
	copy(out, seg)
	return out, total, nil
}

// Update updates a beer in the in-memory repository.
func (r *InMemoryBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, b := range r.beers {
		if b.ID == id {
			beer.ID = id // Ensure ID remains the same
			r.beers[i] = beer
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

// Delete removes a beer from the in-memory repository.
func (r *InMemoryBeerRepository) Delete(ctx context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, b := range r.beers {
		if b.ID == id {
			r.beers = append(r.beers[:i], r.beers[i+1:]...)
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

// AddComment adds a comment to a beer in the in-memory repository.
func (r *InMemoryBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, beer := range r.beers {
		if beer.ID == id {
			r.beers[i].Comments = append(r.beers[i].Comments, comment)
			return nil
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
}

// DeleteComment removes a comment from a beer in the in-memory repository.
func (r *InMemoryBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for i, beer := range r.beers {
		if beer.ID == id {
			for j, c := range beer.Comments {
				if c.ID == commentID {
					r.beers[i].Comments = append(beer.Comments[:j], beer.Comments[j+1:]...)
					return nil
				}
			}
			return errors.NewAppError(404, "comment not found", nil)
		}
	}
	return errors.NewAppError(404, "beer not found", nil)
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
// O id é gerado pelo Postgres (SERIAL); não enviamos beer.ID no INSERT.
// O valor gerado é lido via RETURNING id e escrito de volta em beer.ID (como
// string) para manter o contrato do modelo com os clientes móveis.
func (r *PostgresBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	// Initialize Comments as an empty array if it's nil
	if beer.Comments == nil {
		beer.Comments = []model.Comment{}
	}

	commentsJSON, err := json.Marshal(beer.Comments)
	if err != nil {
		return fmt.Errorf("failed to marshal comments: %w", err)
	}

	var generatedID int64
	// Inserting the beer into the beers table (id auto-gerado pelo SERIAL).
	// created_by regista o dono (AuthZ: só criador ou admin editam/apagam).
	err = r.db.QueryRowContext(ctx, `
   INSERT INTO beers (
       name, style, description, image_url, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by
   ) VALUES ($1, $2, COALESCE($3, NULL), $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
   RETURNING id`,
		beer.Name, beer.Style, beer.Description, beer.ImageUrl, beer.Alcohol, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Finish, commentsJSON, beer.CreatedBy).
		Scan(&generatedID)
	if err != nil {
		return err
	}

	beer.ID = fmt.Sprintf("%d", generatedID)
	return nil
}

// GetByID retrieves a beer by its ID from the PostgreSQL database.
func (r *PostgresBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	var beer model.Beer
	var commentsJSON []byte
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
            created_by
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
			&beer.CreatedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Beer{}, errors.NewAppError(404, "beer not found", nil)
		}
		return model.Beer{}, err
	}

	beer.Comments = unmarshalComments(commentsJSON)
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

func (r *PostgresBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	offset := (page - 1) * pageSize

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
            created_by
        FROM beers LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		slog.Error("error querying beers", "err", err)
		return nil, 0, err
	}
	defer rows.Close()

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
		var commentsJSON []byte
		var description, imageURL, createdBy sql.NullString
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
			&createdBy); err != nil {
			slog.Error("error scanning beer", "err", err)
			return nil, 0, err
		}
		beer.Description = description.String
		beer.ImageUrl = imageURL.String
		beer.CreatedBy = createdBy.String
		beer.Comments = unmarshalComments(commentsJSON)
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
	commentsJSON, err := json.Marshal(beer.Comments)
	if err != nil {
		return fmt.Errorf("failed to marshal comments: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		"UPDATE beers SET name=$1, style=$2, taste=$3, aroma=$4, color=$5, body=$6, carbonation=$7, alcohol=$8, finish=$9, comments=$10 WHERE id=$11",
		beer.Name, beer.Style, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Alcohol, beer.Finish, commentsJSON, id)
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
	qb.WriteString("SELECT id, name, style, description, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by FROM beers WHERE 1=1")
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

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
		var commentsJSON []byte
		var createdBy sql.NullString
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
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan beer: %w", err)
		}
		beer.CreatedBy = createdBy.String
		beer.Comments = unmarshalComments(commentsJSON)
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
		SELECT id, name, style, description, image_url, alcohol, taste, aroma, color, body, carbonation, finish, comments, created_by 
		FROM beers
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get beers: %w", err)
	}
	defer rows.Close()

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
		var commentsJSON []byte
		var description, imageURL, createdBy sql.NullString
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
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan beer: %w", err)
		}
		beer.Description = description.String
		beer.ImageUrl = imageURL.String
		beer.CreatedBy = createdBy.String
		beer.Comments = unmarshalComments(commentsJSON)
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

// UnavailableBeerRepository é um fallback usado quando a base de dados não está
// acessível no arranque. Todas as operações retornam ErrDatabaseUnavailable
// (503), mantendo o servidor de pé para rotas de diagnóstico (/health, /docs).
type UnavailableBeerRepository struct{}

// NewUnavailableBeerRepository cria o repositório de fallback offline.
func NewUnavailableBeerRepository() *UnavailableBeerRepository {
	return &UnavailableBeerRepository{}
}

func (r *UnavailableBeerRepository) GetAll(context.Context) ([]model.Beer, error) {
	return nil, errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) Create(ctx context.Context, beer *model.Beer) error {
	return errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	return nil, 0, errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	return model.Beer{}, errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	return errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) Delete(ctx context.Context, id string) error {
	return errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	return errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) DeleteComment(ctx context.Context, id string, commentID string) error {
	return errors.NewUnavailableError()
}
func (r *UnavailableBeerRepository) SearchBeers(ctx context.Context, filters model.BeerFilters) ([]model.Beer, int, error) {
	return nil, 0, errors.NewUnavailableError()
}
