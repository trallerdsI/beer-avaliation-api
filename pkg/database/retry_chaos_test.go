package database

import (
	"context"
	"database/sql"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"
)

func TestRetryableDB_Chaos_ExecInTx_RecoversAfterDBCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	if !isDockerAvailable() {
		t.Skip("docker daemon not available for chaos tests")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("chaos_test"),
		postgres.WithUsername("chaos"),
		postgres.WithPassword("chaos"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS chaos_accounts (
			id TEXT PRIMARY KEY,
			balance INTEGER NOT NULL
		)
	`)
	require.NoError(t, err)

	retryable := NewRetryableDB(db)
	retryable.SetRetryOptions(5, 50*time.Millisecond)

	tx, err := retryable.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "INSERT INTO chaos_accounts (id, balance) VALUES ($1, $2)", "acc-1", 100)
	require.NoError(t, err)

	timeout := 10 * time.Second
	err = pgContainer.Stop(ctx, &timeout)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "INSERT INTO chaos_accounts (id, balance) VALUES ($1, $2)", "acc-2", 200)
	if err == nil {
		t.Fatal("expected error after container stop, got nil")
	}

	err = tx.Rollback()
	assert.Error(t, err)

	err = pgContainer.Start(ctx)
	require.NoError(t, err)

	connStr, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err = sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer db.Close()

	retryable = NewRetryableDB(db)
	retryable.SetRetryOptions(5, 50*time.Millisecond)

	require.Eventually(t, func() bool {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return db.PingContext(pingCtx) == nil
	}, 30*time.Second, 500*time.Millisecond, "database did not recover in time")

	tx, err = retryable.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "INSERT INTO chaos_accounts (id, balance) VALUES ($1, $2)", "acc-1", 100)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "INSERT INTO chaos_accounts (id, balance) VALUES ($1, $2)", "acc-2", 200)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chaos_accounts").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "expected exactly 2 records, proving first attempt was fully rolled back")
}

func TestRetryableDB_Chaos_NoConnectionLeakAfterCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	if !isDockerAvailable() {
		t.Skip("docker daemon not available for chaos tests")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("leak_test"),
		postgres.WithUsername("leak"),
		postgres.WithPassword("leak"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)
	defer db.Close()

	require.Eventually(t, func() bool {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return db.PingContext(pingCtx) == nil
	}, 30*time.Second, 500*time.Millisecond, "database did not become reachable")

	retryable := NewRetryableDB(db)
	retryable.SetRetryOptions(3, 50*time.Millisecond)

	_, err = retryable.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS leak_test_table (
			id TEXT PRIMARY KEY
		)
	`)
	require.NoError(t, err)

	timeout := 10 * time.Second
	err = pgContainer.Stop(ctx, &timeout)
	require.NoError(t, err)

	attemptCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	err = retryable.PingContext(attemptCtx)
	assert.Error(t, err, "expected ping to fail while container is stopped")

	err = pgContainer.Start(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return db.PingContext(pingCtx) == nil
	}, 60*time.Second, 1*time.Second, "database did not recover in time")

	stats := db.Stats()
	assert.Equal(t, 0, stats.OpenConnections, "expected no leaked connections, got %d", stats.OpenConnections)
}

func isDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}
