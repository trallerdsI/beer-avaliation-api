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
	GetByExternal(ctx context.Context, provider, externalSub string) (model.User, error)
	UpsertByExternal(ctx context.Context, user model.User) (model.User, error)
	Update(ctx context.Context, id string, user model.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, page, pageSize int) ([]model.User, int, error)
	CreatePushSubscription(ctx context.Context, sub model.PushSubscription) error
	ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error)
	DeletePushSubscription(ctx context.Context, id string) error
	DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error
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

func (r *PostgresUserRepository) GetByExternal(ctx context.Context, provider, externalSub string) (model.User, error) {
	var user model.User
	query := `SELECT id, username, email, role, provider, external_sub, created, updated_at FROM beerUsers WHERE provider = $1 AND external_sub = $2`

	err := r.db.QueryRowContext(ctx, query, provider, externalSub).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Role,
		&user.Provider,
		&user.ExternalSub,
		&user.Created,
		&user.UpdatedAt)

	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found")
	}
	if err != nil {
		return user, fmt.Errorf("failed to get user by external: %w", err)
	}
	return user, nil
}

// UpsertByExternal insere ou atualiza uma conta social com base no par
// (provider, external_sub) único. No conflito, atualiza email/username e
// mantém o id/role existentes, garantindo que logins repetidos não duplicam
// utilizadores.
func (r *PostgresUserRepository) UpsertByExternal(ctx context.Context, user model.User) (model.User, error) {
	query := `
		INSERT INTO beerUsers (id, username, email, password, role, provider, external_sub, created)
		VALUES ($1, $2, $3, NULL, $4, $5, $6, $7)
		ON CONFLICT (provider, external_sub) WHERE external_sub IS NOT NULL
		DO UPDATE SET
			email = EXCLUDED.email,
			username = EXCLUDED.username
		RETURNING id, username, email, role, provider, external_sub, created, updated_at`

	var created model.User
	err := r.db.QueryRowContext(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.Role,
		user.Provider,
		user.ExternalSub,
		user.Created,
	).Scan(
		&created.ID,
		&created.Username,
		&created.Email,
		&created.Role,
		&created.Provider,
		&created.ExternalSub,
		&created.Created,
		&created.UpdatedAt)

	if err != nil {
		return model.User{}, fmt.Errorf("failed to upsert external user: %w", err)
	}
	return created, nil
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

func (r *PostgresUserRepository) CreatePushSubscription(ctx context.Context, sub model.PushSubscription) error {
	query := `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, endpoint) DO UPDATE SET
			p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth,
			user_agent = EXCLUDED.user_agent,
			updated_at = now()`

	_, err := r.db.ExecContext(ctx, query,
		sub.UserID,
		sub.Endpoint,
		sub.P256DH,
		sub.Auth,
		sub.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to create push subscription: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	query := `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, updated_at, last_used_at
		FROM push_subscriptions
		WHERE user_id = $1`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []model.PushSubscription
	for rows.Next() {
		var sub model.PushSubscription
		var userAgent, lastUsedAt sql.NullString
		if err := rows.Scan(
			&sub.ID,
			&sub.UserID,
			&sub.Endpoint,
			&sub.P256DH,
			&sub.Auth,
			&userAgent,
			&sub.CreatedAt,
			&sub.UpdatedAt,
			&lastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan push subscription: %w", err)
		}
		sub.UserAgent = userAgent.String
		sub.LastUsedAt = lastUsedAt.String
		subs = append(subs, sub)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating push subscriptions: %w", err)
	}
	return subs, nil
}

func (r *PostgresUserRepository) DeletePushSubscription(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM push_subscriptions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete push subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("push subscription not found")
	}
	return nil
}

func (r *PostgresUserRepository) DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2", userID, endpoint)
	if err != nil {
		return fmt.Errorf("failed to delete push subscription by endpoint: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("push subscription not found")
	}
	return nil
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
func (r *UnavailableUserRepository) GetByExternal(ctx context.Context, provider, externalSub string) (model.User, error) {
	return model.User{}, appErrors.NewUnavailableError()
}
func (r *UnavailableUserRepository) UpsertByExternal(ctx context.Context, user model.User) (model.User, error) {
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

func (r *UnavailableUserRepository) CreatePushSubscription(ctx context.Context, sub model.PushSubscription) error {
	return appErrors.NewUnavailableError()
}

func (r *UnavailableUserRepository) ListPushSubscriptions(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	return nil, appErrors.NewUnavailableError()
}

func (r *UnavailableUserRepository) DeletePushSubscription(ctx context.Context, id string) error {
	return appErrors.NewUnavailableError()
}

func (r *UnavailableUserRepository) DeletePushSubscriptionByEndpoint(ctx context.Context, userID, endpoint string) error {
	return appErrors.NewUnavailableError()
}
