package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beer-review-app/internal/user/model"
	appErrors "beer-review-app/pkg/errors"
)

type UserRepository interface {
	Create(ctx context.Context, user model.User) error
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	Update(ctx context.Context, id string, user model.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page, pageSize int) ([]model.User, int, error)
}

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) (UserRepository, error) {
	if db == nil {
		return nil, appErrors.NewUnavailableError()
	}
	if err := db.Ping(); err != nil {
		return nil, appErrors.NewAppError(503, "user database unavailable", err)
	}
	return &PostgresUserRepository{db: db}, nil
}

func (r *PostgresUserRepository) Create(ctx context.Context, user model.User) error {
	// id é um UUIDv7 (RFC 9562) gerado pela aplicação e já preenchido em
	// user.ID antes da chamada (ver userUsecase.Register/SeedAdmin).
	query := `
		INSERT INTO beerUsers (id, username, email, password, role, created)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.Password,
		user.Role,
		user.Created)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	var user model.User
	query := `SELECT id, username, email, role, created, updated_at FROM beerUsers WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Role,
		&user.Created,
		&user.UpdatedAt)

	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found")
	}
	if err != nil {
		return user, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	query := `SELECT id, username, email, password, role, created, updated_at FROM beerUsers WHERE email = $1`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.Created,
		&user.UpdatedAt)

	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found")
	}
	if err != nil {
		return user, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (r *PostgresUserRepository) Update(ctx context.Context, id string, user model.User) error {
	query := `
		UPDATE beerUsers 
		SET username = $1, email = $2
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query,
		user.Username,
		user.Email,
		id)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *PostgresUserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM beerUsers WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *PostgresUserRepository) List(ctx context.Context, page, pageSize int) ([]model.User, int, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	countErr := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beerUsers").Scan(&total)
	if countErr != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", countErr)
	}

	// Get paginated users
	query := `
		SELECT id, username, email, role, created 
		FROM beerUsers 
		ORDER BY created DESC 
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []model.User = make([]model.User, 0)
	for rows.Next() {
		var user model.User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Role,
			&user.Created); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over user rows: %w", err)
	}

	return users, total, nil
}

// UnavailableUserRepository é o fallback offline quando a base de dados não
// está acessível. Todas as operações retornam ErrDatabaseUnavailable (503).
type UnavailableUserRepository struct{}

// NewUnavailableUserRepository cria o repositório de fallback offline.
func NewUnavailableUserRepository() *UnavailableUserRepository {
	return &UnavailableUserRepository{}
}

func (r *UnavailableUserRepository) Create(ctx context.Context, user model.User) error {
	return appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	return model.User{}, appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	return model.User{}, appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) Update(ctx context.Context, id string, user model.User) error {
	return appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) Delete(ctx context.Context, id string) error {
	return appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) List(ctx context.Context, page, pageSize int) ([]model.User, int, error) {
	return nil, 0, appErrors.NewUnavailableError()
}
