package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beer-review-app/internal/user/model"
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

func NewPostgresUserRepository(db *sql.DB) UserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user model.User) error {
	query := `
		INSERT INTO beerUsers (id, username, email, password, created)
		VALUES ($1, $2, $3, $4, $5)`
	
	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.Password,
		user.Created)
	
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	var user model.User
	query := `SELECT id, username, email, created FROM beerUsers WHERE id = $1`
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Created)
	
	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found")
	}
	if err != nil {
		return user, fmt.Errorf("failed to get user: %v", err)
	}
	return user, nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	query := `SELECT id, username, email, password, created FROM beerUsers WHERE email = $1`
	
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Created)
	
	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found")
	}
	if err != nil {
		return user, fmt.Errorf("failed to get user: %v", err)
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
		return fmt.Errorf("failed to update user: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %v", err)
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
		return fmt.Errorf("failed to delete user: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %v", err)
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
		return nil, 0, fmt.Errorf("failed to count users: %v", countErr)
	}
	
	// Get paginated users
	query := `
		SELECT id, username, email, created 
		FROM beerUsers 
		ORDER BY created DESC 
		LIMIT $1 OFFSET $2`
	
	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %v", err)
	}
	defer rows.Close()
	
	var users []model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Created); err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, user)
	}
	
	return users, total, nil
}
