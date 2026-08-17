package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beer-review-app/internal/beer/model"
	"beer-review-app/pkg/errors"
)

type ReportRepository interface {
	CreateReport(ctx context.Context, report *model.BeerReport) error
	GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error)
	GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error)
	ResolveReport(ctx context.Context, reportID, status string, resolvedBy *string, resolvedAt time.Time) error
}

type DeletionRequestRepository interface {
	CreateDeletionRequest(ctx context.Context, req *model.BeerDeletionRequest) error
	GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error)
	ResolveDeletionRequest(ctx context.Context, reqID, status string, reviewedBy *string, reviewedAt time.Time) error
}

type ModerationRepository interface {
	CreateReport(ctx context.Context, report *model.BeerReport) error
	GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error)
	GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error)
	ResolveReport(ctx context.Context, reportID, status string, resolvedBy *string, resolvedAt time.Time) error
	CreateDeletionRequest(ctx context.Context, req *model.BeerDeletionRequest) error
	GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error)
	ResolveDeletionRequest(ctx context.Context, reqID, status string, reviewedBy *string, reviewedAt time.Time) error
	ExecInTx(ctx context.Context, fn func(ctx context.Context, txRepo ModerationRepository, txBeerRepo BeerRepository) error) error
}

type PostgresModerationRepository struct {
	db querier
}

func NewPostgresModerationRepository(db querier) (*PostgresModerationRepository, error) {
	if isNilQuerier(db) {
		return nil, errors.NewUnavailableError()
	}
	if err := db.(interface{ Ping() error }).Ping(); err != nil {
		return nil, errors.NewAppError(503, "moderation database unavailable", err)
	}
	return &PostgresModerationRepository{db: db}, nil
}

func (r *PostgresModerationRepository) withTx(tx *sql.Tx) *PostgresModerationRepository {
	return &PostgresModerationRepository{db: tx}
}

func (r *PostgresModerationRepository) CreateReport(ctx context.Context, report *model.BeerReport) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO beer_reports (id, beer_id, user_id, reason, description, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, report.ID, report.BeerID, report.UserID, report.Reason, report.Description, report.Status, report.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresModerationRepository) GetReportsByBeerID(ctx context.Context, beerID string, limit, offset int) ([]model.BeerReport, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beer_reports WHERE beer_id = $1", beerID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, beer_id, user_id, reason, description, status, created_at, resolved_by, resolved_at
		FROM beer_reports
		WHERE beer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, beerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []model.BeerReport
	for rows.Next() {
		var report model.BeerReport
		var resolvedBy sql.NullString
		var resolvedAt sql.NullTime
		err := rows.Scan(
			&report.ID, &report.BeerID, &report.UserID, &report.Reason, &report.Description, &report.Status,
			&report.CreatedAt, &resolvedBy, &resolvedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if resolvedBy.Valid {
			v := resolvedBy.String
			report.ResolvedBy = &v
		}
		if resolvedAt.Valid {
			v := resolvedAt.Time
			report.ResolvedAt = &v
		}
		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *PostgresModerationRepository) GetReports(ctx context.Context, filter model.ReportFilter) ([]model.BeerReport, int, error) {
	args := []interface{}{}
	argPos := 1
	query := "SELECT id, beer_id, user_id, reason, description, status, created_at, resolved_by, resolved_at FROM beer_reports WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM beer_reports WHERE 1=1"

	if filter.BeerID != "" {
		query += fmt.Sprintf(" AND beer_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND beer_id = $%d", argPos)
		args = append(args, filter.BeerID)
		argPos++
	}
	if filter.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, filter.UserID)
		argPos++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		countQuery += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, filter.Status)
		argPos++
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query += " ORDER BY created_at DESC LIMIT " + fmt.Sprintf("$%d", argPos) + " OFFSET " + fmt.Sprintf("$%d", argPos+1)
	args = append(args, filter.Limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []model.BeerReport
	for rows.Next() {
		var report model.BeerReport
		var resolvedBy sql.NullString
		var resolvedAt sql.NullTime
		if err := rows.Scan(
			&report.ID, &report.BeerID, &report.UserID, &report.Reason, &report.Description, &report.Status,
			&report.CreatedAt, &resolvedBy, &resolvedAt,
		); err != nil {
			return nil, 0, err
		}
		if resolvedBy.Valid {
			v := resolvedBy.String
			report.ResolvedBy = &v
		}
		if resolvedAt.Valid {
			v := resolvedAt.Time
			report.ResolvedAt = &v
		}
		reports = append(reports, report)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *PostgresModerationRepository) ResolveReport(ctx context.Context, reportID, status string, resolvedBy *string, resolvedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE beer_reports
		SET status = $1, resolved_by = $2, resolved_at = $3
		WHERE id = $4
	`, status, resolvedBy, resolvedAt, reportID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.NewAppError(404, "report not found", nil)
	}
	return nil
}

func (r *PostgresModerationRepository) CreateDeletionRequest(ctx context.Context, req *model.BeerDeletionRequest) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO beer_deletion_requests (id, beer_id, user_id, reason, details, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, req.ID, req.BeerID, req.UserID, req.Reason, req.Details, req.Status, req.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresModerationRepository) GetDeletionRequests(ctx context.Context, filter model.DeletionRequestFilter) ([]model.BeerDeletionRequest, int, error) {
	args := []interface{}{}
	argPos := 1
	query := "SELECT id, beer_id, user_id, reason, details, status, created_at, reviewed_by, reviewed_at FROM beer_deletion_requests WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM beer_deletion_requests WHERE 1=1"

	if filter.BeerID != "" {
		query += fmt.Sprintf(" AND beer_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND beer_id = $%d", argPos)
		args = append(args, filter.BeerID)
		argPos++
	}
	if filter.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argPos)
		countQuery += fmt.Sprintf(" AND user_id = $%d", argPos)
		args = append(args, filter.UserID)
		argPos++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		countQuery += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, filter.Status)
		argPos++
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query += " ORDER BY created_at DESC LIMIT " + fmt.Sprintf("$%d", argPos) + " OFFSET " + fmt.Sprintf("$%d", argPos+1)
	args = append(args, filter.Limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []model.BeerDeletionRequest
	for rows.Next() {
		var req model.BeerDeletionRequest
		var reviewedBy sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(
			&req.ID, &req.BeerID, &req.UserID, &req.Reason, &req.Details, &req.Status,
			&req.CreatedAt, &reviewedBy, &reviewedAt,
		); err != nil {
			return nil, 0, err
		}
		if reviewedBy.Valid {
			v := reviewedBy.String
			req.ReviewedBy = &v
		}
		if reviewedAt.Valid {
			v := reviewedAt.Time
			req.ReviewedAt = &v
		}
		requests = append(requests, req)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return requests, total, nil
}

func (r *PostgresModerationRepository) ResolveDeletionRequest(ctx context.Context, reqID, status string, reviewedBy *string, reviewedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE beer_deletion_requests
		SET status = $1, reviewed_by = $2, reviewed_at = $3
		WHERE id = $4
	`, status, reviewedBy, reviewedAt, reqID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.NewAppError(404, "deletion request not found", nil)
	}
	return nil
}

func (r *PostgresModerationRepository) ExecInTx(ctx context.Context, fn func(ctx context.Context, txRepo ModerationRepository, txBeerRepo BeerRepository) error) error {
	db, ok := r.db.(*sql.DB)
	if !ok {
		return errors.NewUnavailableError()
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txModerationRepo := r.withTx(tx)
	txBeerRepo := &PostgresBeerRepository{db: tx}

	if err := fn(ctx, txModerationRepo, txBeerRepo); err != nil {
		return err
	}

	return tx.Commit()
}
