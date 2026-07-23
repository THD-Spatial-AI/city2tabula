//go:build integration

package db_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// writeSQLScript writes a one-off SQL file for ExecuteCityDBScript to run, using
// real psql against the shared testPool's database rather than mocking psql's
// output - executeCityDBScript's branching (success / "already exists" / generic
// failure) depends on what real psql actually prints, which is what's under test.
func writeSQLScript(t *testing.T, sql string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sql")
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		t.Fatalf("failed to write SQL script: %v", err)
	}
	return path
}

func TestExecuteCityDBScript_Success(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	const schema = "citydb_script_success_test"
	defer testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)

	script := writeSQLScript(t, `CREATE SCHEMA `+schema+`;`)

	if err := db.ExecuteCityDBScript(cfg, script, ""); err != nil {
		t.Fatalf("ExecuteCityDBScript: %v", err)
	}
	if !schemaExists(t, ctx, schema) {
		t.Errorf("expected schema %q to exist after a successful script run", schema)
	}
}

// TestExecuteCityDBScript_AlreadyExistsIsDetected drives the branch that inspects
// psql's real output for "already exists": running the same plain CREATE SCHEMA
// (no IF NOT EXISTS) a second time makes Postgres itself raise that exact error,
// rather than simulating the message.
func TestExecuteCityDBScript_AlreadyExistsIsDetected(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	const schema = "citydb_script_exists_test"
	defer testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)

	script := writeSQLScript(t, `CREATE SCHEMA `+schema+`;`)

	if err := db.ExecuteCityDBScript(cfg, script, ""); err != nil {
		t.Fatalf("first run (should succeed): %v", err)
	}

	err := db.ExecuteCityDBScript(cfg, script, "")
	if err == nil {
		t.Fatal("expected an error on the second run (schema already exists), got nil")
	}
	if !strings.Contains(err.Error(), "schema already exists") {
		t.Errorf("expected the already-exists branch to be detected, got: %v", err)
	}
}

// TestExecuteCityDBScript_GenericFailureIsNotMisreportedAsAlreadyExists exercises
// the other side of that same branch: a script that fails for an unrelated reason
// must not be mislabeled as "schema already exists".
func TestExecuteCityDBScript_GenericFailureIsNotMisreportedAsAlreadyExists(t *testing.T) {
	cfg := testConfig("citytabula_dbtest")
	script := writeSQLScript(t, `THIS IS NOT VALID SQL;`)

	err := db.ExecuteCityDBScript(cfg, script, "")
	if err == nil {
		t.Fatal("expected an error for invalid SQL, got nil")
	}
	if strings.Contains(err.Error(), "schema already exists") {
		t.Errorf("expected a generic failure, got it misreported as already-exists: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to execute CityDB script") {
		t.Errorf("expected the generic-failure wrapper message, got: %v", err)
	}
}

// TestExecuteCityDBScript_WithSchemaNameVariable confirms the schema_name psql
// variable is only added to args when schemaName is non-empty (the WHERE
// schemaName != "" branch in executeCityDBScript), using a script that
// references :schema_name so a missing/wrong variable would fail the run.
func TestExecuteCityDBScript_WithSchemaNameVariable(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	const schema = "citydb_script_var_test"
	defer testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)

	script := writeSQLScript(t, `CREATE SCHEMA :schema_name;`)

	if err := db.ExecuteCityDBScript(cfg, script, schema); err != nil {
		t.Fatalf("ExecuteCityDBScript with schemaName: %v", err)
	}
	if !schemaExists(t, ctx, schema) {
		t.Errorf("expected schema %q (from the schema_name psql variable) to exist", schema)
	}
}
