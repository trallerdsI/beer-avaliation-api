package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryableDB wraps *sql.DB with retry/backoff for transient errors.
type RetryableDB struct {
	*sql.DB
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryableDB creates a RetryableDB with default settings.
func NewRetryableDB(db *sql.DB) *RetryableDB {
	return &RetryableDB{
		DB:         db,
		maxRetries: 3,
		baseDelay:  100 * time.Millisecond,
	}
}

// SetRetryOptions configures retry parameters.
func (r *RetryableDB) SetRetryOptions(maxRetries int, baseDelay time.Duration) {
	r.maxRetries = maxRetries
	r.baseDelay = baseDelay
}

// IsTransientError returns true if the error is a transient database error
// that should be retried (e.g., connection resets, bad connection, PostgreSQL admin shutdown).
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	if err == driver.ErrBadConn {
		return true
	}

	if strings.Contains(err.Error(), "driver: bad connection") {
		return true
	}

	if strings.Contains(err.Error(), "sql: database is closed") {
		return true
	}

	if strings.Contains(err.Error(), "connection reset") {
		return true
	}

	if strings.Contains(err.Error(), "broken pipe") {
		return true
	}

	if strings.Contains(err.Error(), "57P01") {
		return true
	}

	return false
}

// retryExec executes a function with exponential backoff + jitter.
func retryExec[T any](ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() (T, error)) (T, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		if !IsTransientError(err) {
			return result, err
		}

		lastErr = err

		if attempt < maxRetries {
			jitter := time.Duration(rand.Int63n(int64(baseDelay)))
			delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay + jitter

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			}
		}
	}

	var zero T
	return zero, lastErr
}

// ExecContext executes a query with retry logic.
func (r *RetryableDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return retryExec(ctx, r.maxRetries, r.baseDelay, func() (sql.Result, error) {
		return r.DB.ExecContext(ctx, query, args...)
	})
}

// QueryContext executes a query with retry logic.
func (r *RetryableDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return retryExec(ctx, r.maxRetries, r.baseDelay, func() (*sql.Rows, error) {
		return r.DB.QueryContext(ctx, query, args...)
	})
}

// QueryRowContext executes a query row with retry logic.
func (r *RetryableDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	row, err := retryExec(ctx, r.maxRetries, r.baseDelay, func() (*sql.Row, error) {
		return r.DB.QueryRowContext(ctx, query, args...), nil
	})
	if err != nil {
		return r.DB.QueryRowContext(ctx, query, args...)
	}
	return row
}

// BeginTx starts a transaction with retry logic.
func (r *RetryableDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return retryExec(ctx, r.maxRetries, r.baseDelay, func() (*sql.Tx, error) {
		return r.DB.BeginTx(ctx, opts)
	})
}

// PingContext pings the database with retry logic.
func (r *RetryableDB) PingContext(ctx context.Context) error {
	_, err := retryExec(ctx, r.maxRetries, r.baseDelay, func() (struct{}, error) {
		return struct{}{}, r.DB.PingContext(ctx)
	})
	return err
}
