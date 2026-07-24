//go:build integration

package db_test

import (
	"strings"
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

// TestConnectPool_ParseConfigFailure covers ConnectPool's own "parse pool
// config failed" wrap: EnsureDatabase succeeds against the real bootstrap
// DB (real host/port), then a negative Batch.Threads makes the DSN's
// pool_max_conns=%d segment parse as an invalid (too small) value, failing
// pgxpool.ParseConfig itself - no unreachable target DB needed.
func TestConnectPool_ParseConfigFailure(t *testing.T) {
	cfg := testConfig("connect_pool_parseconfig_test")
	cfg.Batch.Threads = -1

	pool, err := db.ConnectPool(cfg)
	if err == nil {
		t.Fatal("expected a parse pool config error for a negative pool_max_conns, got nil")
	}
	if !strings.Contains(err.Error(), "parse pool config failed") {
		t.Errorf("expected ConnectPool's own error wrap, got: %v", err)
	}
	if pool != nil {
		t.Error("expected a nil pool on failure")
	}
}
