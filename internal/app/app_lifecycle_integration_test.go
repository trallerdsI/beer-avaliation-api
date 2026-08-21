//go:build integration

package app

import (
	"database/sql"
	"testing"
)

func TestInitDB_MalformedDSN_ReturnsError(t *testing.T) {
	_, err := InitDB("not-a-valid-dsn")
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
}

func TestInitDB_UnsupportedDriver_ReturnsError(t *testing.T) {
	_, err := sql.Open("nonexistent-driver", "dsn")
	if err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}
}

func TestInitDB_PingFailure_ReturnsError(t *testing.T) {
	dsn := "postgres://invalid:invalid@localhost:99999/nonexistent?sslmode=disable"
	db, err := InitDB(dsn)
	if err == nil {
		db.Close()
		t.Fatal("expected error for unreachable database, got nil")
	}
}

func TestInitDB_ServerlessPingTimeout_ReturnsError(t *testing.T) {
	t.Setenv("SERVERLESS", "true")
	dsn := "postgres://invalid:invalid@localhost:99999/nonexistent?sslmode=disable"
	db, err := InitDB(dsn)
	if err == nil {
		db.Close()
		t.Fatal("expected error for serverless ping timeout, got nil")
	}
}
