package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"

	_ "github.com/lib/pq"
)

// BeerRepository defines the interface for beer storage.
// In your repository package (repository/beer_repository.go)
type BeerRepository interface {
	GetAll(context.Context) ([]model.Beer, error)
	Create(ctx context.Context, beer model.Beer) error
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
func (r *InMemoryBeerRepository) Create(ctx context.Context, beer model.Beer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.beers = append(r.beers, beer)
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

// GetAll retrieves all beers from the in-memory repository with pagination.
func (r *InMemoryBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.beers, nil
}

// GetPaginated retrieves paginated beers from the in-memory repository
func (r *InMemoryBeerRepository) GetPaginated(ctx context.Context, page, pageSize int) ([]model.Beer, int, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	start := (page - 1) * pageSize
	end := start + pageSize
	total := len(r.beers)

	if start >= total {
		return []model.Beer{}, total, nil
	}

	if end > total {
		end = total
	}

	return r.beers[start:end], total, nil
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
	// Check if the connection is valid
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresBeerRepository{db: db}, nil
}

// Create adds a new beer to the PostgreSQL database.
func (r *PostgresBeerRepository) Create(ctx context.Context, beer model.Beer) error {
	// Initialize Comments as an empty array if it's nil
	if beer.Comments == nil {
		beer.Comments = []model.Comment{}
	}

	// Inserting the beer into the beers table
	result, err := r.db.ExecContext(ctx, `
   INSERT INTO beers (
        id, name, style, description, image_url, alcohol, taste, aroma, color, body, carbonation, finish
    ) VALUES ($1, $2, $3, COALESCE($4, NULL), $5, $6, $7, $8, $9, $10, $11, $12)`,
		beer.ID, beer.Name, beer.Style, beer.Description, beer.ImageUrl, beer.Alcohol, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Finish)
	if err != nil {
		return err
	}

	// Check if the insert was successful
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.NewAppError(500, "no rows were inserted", nil)
	}

	// Insert comments if any (empty array here means no comments to insert)
	for _, comment := range beer.Comments {
		_, err := r.db.ExecContext(ctx, `
        INSERT INTO comments (beer_id, text)
        VALUES ($1, $2)`, beer.ID, comment.Text)
		if err != nil {
			return fmt.Errorf("failed to insert comment: %v", err)
		}
	}

	return nil
}

// GetByID retrieves a beer by its ID from the PostgreSQL database.
func (r *PostgresBeerRepository) GetByID(ctx context.Context, id string) (model.Beer, error) {
	var beer model.Beer
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
            finish
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
			&beer.Finish)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.Beer{}, errors.NewAppError(404, "beer not found", nil)
		}
		return model.Beer{}, err
	}
	return beer, nil
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
            finish
        FROM beers LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		log.Printf("Error querying beers: %v", err) // Log de erro
		return nil, 0, err
	}
	defer rows.Close()

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
		if err := rows.Scan(
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
			&beer.Finish); err != nil {
			log.Printf("Error scanning beer: %v", err) // Log de erro
			return nil, 0, err
		}
		beers = append(beers, beer)
	}

	// Verificando erros após iterar sobre as linhas
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating over rows: %v", err) // Log de erro
		return nil, 0, err
	}

	// Obtendo a contagem total de cervejas
	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beers").Scan(&total)
	if err != nil {
		log.Printf("Error getting total beer count: %v", err) // Log de erro
		return nil, 0, err
	}

	return beers, total, nil
}

// Update updates a beer in the PostgreSQL database.
func (r *PostgresBeerRepository) Update(ctx context.Context, id string, beer model.Beer) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE beers SET name=$1, style=$2, taste=$3, aroma=$4, color=$5, body=$6, carbonation=$7, alcohol=$8, finish=$9 WHERE id=$10",
		beer.Name, beer.Style, beer.Taste, beer.Aroma, beer.Color, beer.Body, beer.Carbonation, beer.Alcohol, beer.Finish, id)
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
	query := `
        SELECT id, name, style, description, alcohol, taste, aroma, color, body, carbonation, finish
        FROM beers
        WHERE 1=1
    `
	countQuery := `SELECT COUNT(*) FROM beers WHERE 1=1`
	args := []interface{}{}
	argPosition := 1

	// Add filters dynamically to query
	if filters.Query != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argPosition, argPosition)
		countQuery += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argPosition, argPosition)
		args = append(args, "%"+filters.Query+"%")
		argPosition++
	}

	if filters.Style != "" {
		query += fmt.Sprintf(" AND style = $%d", argPosition)
		countQuery += fmt.Sprintf(" AND style = $%d", argPosition)
		args = append(args, filters.Style)
		argPosition++
	}

	if filters.MinAlcohol != nil {
		query += fmt.Sprintf(" AND alcohol >= $%d", argPosition)
		countQuery += fmt.Sprintf(" AND alcohol >= $%d", argPosition)
		args = append(args, *filters.MinAlcohol)
		argPosition++
	}

	if filters.MaxAlcohol != nil {
		query += fmt.Sprintf(" AND alcohol <= $%d", argPosition)
		countQuery += fmt.Sprintf(" AND alcohol <= $%d", argPosition)
		args = append(args, *filters.MaxAlcohol)
		argPosition++
	}

	if filters.Taste != "" {
		query += fmt.Sprintf(" AND taste = $%d", argPosition)
		countQuery += fmt.Sprintf(" AND taste = $%d", argPosition)
		args = append(args, filters.Taste)
		argPosition++
	}

	// Get total count of filtered beers
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count beers: %v", err)
	}

	// Add pagination
	query += fmt.Sprintf(" ORDER BY name LIMIT $%d OFFSET $%d", argPosition, argPosition+1)
	args = append(args, filters.PageSize, (filters.Page-1)*filters.PageSize)

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search beers: %v", err)
	}
	defer rows.Close()

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
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
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan beer: %v", err)
		}
		beers = append(beers, beer)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over rows: %v", err)
	}

	return beers, total, nil
}

// GetAll retrieves all beers from the PostgreSQL database.
func (r *PostgresBeerRepository) GetAll(ctx context.Context) ([]model.Beer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, style, description, alcohol, taste, aroma, color, body, carbonation, finish 
		FROM beers
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get beers: %v", err)
	}
	defer rows.Close()

	var beers []model.Beer
	for rows.Next() {
		var beer model.Beer
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
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan beer: %v", err)
		}
		beers = append(beers, beer)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return beers, nil
}

// AddComment adds a comment to a beer in the PostgreSQL database.
func (r *PostgresBeerRepository) AddComment(ctx context.Context, id string, comment model.Comment) error {
	// First get the beer to ensure it exists
	beer, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get beer: %v", err)
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
		return fmt.Errorf("failed to get beer: %v", err)
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
