package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beer-review-app/pkg/errors"
)

// GetMemberSince devolve a data de criação da conta do utilizador.
func (r *PostgresUserRepository) GetMemberSince(ctx context.Context, userID string) (time.Time, error) {
	var createdAt time.Time
	if err := scanRow(ctx, r.db, `
		SELECT created FROM beerUsers WHERE id = $1`, []any{userID}, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, errors.NewAppError(404, "user not found", err)
		}
		return time.Time{}, fmt.Errorf("failed to get member since: %w", err)
	}
	return createdAt, nil
}
