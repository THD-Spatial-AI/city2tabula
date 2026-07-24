//go:build integration

package db_test

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// TestEnsureDatabase_ConnectFailure drives the pgx.Connect error branch with a
// real (unreachable) host, rather than simulating a connection error.
func TestEnsureDatabase_ConnectFailure(t *testing.T) {
	cfg := testConfig("irrelevant")
	cfg.DB.Host = "127.0.0.1"
	cfg.DB.Port = "1" // nothing listens on port 1

	if err := db.EnsureDatabase(cfg); err == nil {
		t.Fatal("expected a connection error for an unreachable bootstrap host, got nil")
	}
}

// TestEnsureDatabase_CreateDatabaseFailure drives the CREATE DATABASE error
// branch for real: the exists-check passes (this name has never been used),
// but a double quote embedded in the name breaks CREATE DATABASE "<name>"'s
// quoting, so Postgres itself rejects it as invalid syntax.
func TestEnsureDatabase_CreateDatabaseFailure(t *testing.T) {
	cfg := testConfig(`bad"name`)

	if err := db.EnsureDatabase(cfg); err == nil {
		t.Fatal("expected an error for a database name that breaks CREATE DATABASE's quoting, got nil")
	}
}

// TestConnectPool_PropagatesEnsureDatabaseFailure covers ConnectPool's direct
// passthrough of EnsureDatabase's error (no wrap, so this also protects
// against wrapping being added later without a matching test update).
func TestConnectPool_PropagatesEnsureDatabaseFailure(t *testing.T) {
	cfg := testConfig("irrelevant")
	cfg.DB.Host = "127.0.0.1"
	cfg.DB.Port = "1"

	pool, err := db.ConnectPool(cfg)
	if err == nil {
		t.Fatal("expected ConnectPool to fail when EnsureDatabase fails, got nil")
	}
	if pool != nil {
		t.Error("expected a nil pool on failure")
	}
}
