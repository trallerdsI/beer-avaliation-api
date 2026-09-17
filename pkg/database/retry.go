package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	defaultMaxRetries = 3
	defaultBaseDelay  = 100 * time.Millisecond
	envMaxRetries     = "RETRY_MAX_RETRIES"
	envBaseDelay      = "RETRY_BASE_DELAY"
)

var DBRetryAttemptsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "db_retry_attempts_total",
		Help: "Total number of database retry attempts by operation",
	},
	[]string{"operation"},
)

// RetryableDB wraps *sql.DB with retry/backoff for transient errors.
type RetryableDB struct {
	*sql.DB
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryableDB creates a RetryableDB reading configuration from environment
// variables when available. It falls back to safe defaults when the variables
// are unset or malformed, so it never panics during startup.
func NewRetryableDB(db *sql.DB) *RetryableDB {
	r := &RetryableDB{
		DB:         db,
		maxRetries: defaultMaxRetries,
		baseDelay:  defaultBaseDelay,
	}
	r.loadEnvConfig()
	return r
}

func (r *RetryableDB) loadEnvConfig() {
	if v := os.Getenv(envMaxRetries); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			r.maxRetries = n
		}
	}
	if v := os.Getenv(envBaseDelay); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			r.baseDelay = d
		}
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
func retryExec[T any](ctx context.Context, maxRetries int, baseDelay time.Duration, operation string, fn func() (T, error)) (T, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}

		_, span := otel.Tracer("database").Start(ctx, "db."+operation)
		result, err := fn()
		span.End()

		if err == nil {
			return result, nil
		}

		if !IsTransientError(err) {
			return result, err
		}

		lastErr = err

		if attempt < maxRetries {
			DBRetryAttemptsTotal.WithLabelValues(operation).Inc()
			var jitter time.Duration
			if baseDelay > 0 {
				jitter = time.Duration(rand.Int63n(int64(baseDelay))) //nosec
			}
			delay := time.Duration(math.Pow(2, float64(attempt)))*baseDelay + jitter

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
	return retryExec(ctx, r.maxRetries, r.baseDelay, "exec", func() (sql.Result, error) {
		return r.DB.ExecContext(ctx, query, args...)
	})
}

// QueryContext executes a query with retry logic.
func (r *RetryableDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return retryExec(ctx, r.maxRetries, r.baseDelay, "query", func() (*sql.Rows, error) {
		return r.DB.QueryContext(ctx, query, args...)
	})
}

// QueryRowContext executes a query row with retry logic.
func (r *RetryableDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return r.DB.QueryRowContext(ctx, query, args...)
}

// ScanRowContext retries both query execution and scanning. QueryRowContext
// cannot observe database errors because database/sql exposes them only from
// Scan, after the row has already been returned.
func (r *RetryableDB) ScanRowContext(ctx context.Context, query string, args []any, dest ...any) error {
	_, err := retryExec(ctx, r.maxRetries, r.baseDelay, "queryrow", func() (struct{}, error) {
		rows, err := r.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return struct{}{}, err
		}
		defer rows.Close()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return struct{}{}, err
			}
			return struct{}{}, sql.ErrNoRows
		}
		if err := rows.Scan(dest...); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, rows.Err()
	})
	return err
}

// BeginTx starts a transaction with retry logic.
func (r *RetryableDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return retryExec(ctx, r.maxRetries, r.baseDelay, "begintx", func() (*sql.Tx, error) {
		return r.DB.BeginTx(ctx, opts)
	})
}

// PingContext pings the database with retry logic.
func (r *RetryableDB) PingContext(ctx context.Context) error {
	_, err := retryExec(ctx, r.maxRetries, r.baseDelay, "ping", func() (struct{}, error) {
		return struct{}{}, r.DB.PingContext(ctx)
	})
	return err
}
