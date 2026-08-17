package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ErrBadConn",
			err:      driver.ErrBadConn,
			expected: true,
		},
		{
			name:     "bad connection string",
			err:      errors.New("driver: bad connection"),
			expected: true,
		},
		{
			name:     "database is closed",
			err:      errors.New("sql: database is closed"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("write: broken pipe"),
			expected: true,
		},
		{
			name:     "PostgreSQL admin shutdown",
			err:      errors.New(`ERROR: 57P01: admin_shutdown`),
			expected: true,
		},
		{
			name:     "non-transient error",
			err:      errors.New("syntax error"),
			expected: false,
		},
		{
			name:     "constraint violation",
			err:      errors.New("UNIQUE constraint violated"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransientError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryExec_SucceedsOnFirstTry(t *testing.T) {
	var callCount int
	fn := func() (int, error) {
		callCount++
		return 42, nil
	}

	result, err := retryExec(context.Background(), 3, 10*time.Millisecond, fn)
	assert.NoError(t, err)
	assert.Equal(t, 42, result)
	assert.Equal(t, 1, callCount)
}

func TestRetryExec_RetriesOnTransientError(t *testing.T) {
	var callCount int
	fn := func() (int, error) {
		callCount++
		if callCount < 3 {
			return 0, driver.ErrBadConn
		}
		return 42, nil
	}

	result, err := retryExec(context.Background(), 3, 10*time.Millisecond, fn)
	assert.NoError(t, err)
	assert.Equal(t, 42, result)
	assert.Equal(t, 3, callCount)
}

func TestRetryExec_ReturnsNonTransientError(t *testing.T) {
	fn := func() (int, error) {
		return 0, errors.New("syntax error")
	}

	result, err := retryExec(context.Background(), 3, 10*time.Millisecond, fn)
	assert.Error(t, err)
	assert.Equal(t, 0, result)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestRetryExec_RespectsMaxRetries(t *testing.T) {
	var callCount int
	fn := func() (int, error) {
		callCount++
		return 0, driver.ErrBadConn
	}

	result, err := retryExec(context.Background(), 2, 10*time.Millisecond, fn)
	assert.Error(t, err)
	assert.Equal(t, 0, result)
	assert.Equal(t, 3, callCount) // initial + 2 retries
}

func TestRetryExec_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var callCount int
	fn := func() (int, error) {
		callCount++
		return 0, driver.ErrBadConn
	}

	result, err := retryExec(ctx, 3, 10*time.Millisecond, fn)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 0, result)
	assert.Equal(t, 0, callCount) // should not even call fn
}

func TestNewRetryableDB(t *testing.T) {
	var nilDB *sql.DB
	retryable := NewRetryableDB(nilDB)
	assert.NotNil(t, retryable)
	assert.Equal(t, 3, retryable.maxRetries)
	assert.Equal(t, 100*time.Millisecond, retryable.baseDelay)
	assert.Equal(t, nilDB, retryable.DB)
}

func TestRetryableDB_SetRetryOptions(t *testing.T) {
	var nilDB *sql.DB
	retryable := NewRetryableDB(nilDB)
	retryable.SetRetryOptions(5, 200*time.Millisecond)
	assert.Equal(t, 5, retryable.maxRetries)
	assert.Equal(t, 200*time.Millisecond, retryable.baseDelay)
}
