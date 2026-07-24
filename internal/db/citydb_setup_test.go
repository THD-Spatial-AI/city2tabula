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

// writeSQLFixture writes sql to a temp file and returns its absolute path -
// unlike the real 3dcitydb-tool's CreateDB/CreateSchema scripts (not present
// in this repo), these are self-authored, harmless stand-ins: CreateCityDB
// only cares that ExecuteCityDBScript can run *some* SQL file via real psql,
// not what that SQL actually does.
func writeSQLFixture(t *testing.T, sql string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.sql")
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		t.Fatalf("failed to write SQL fixture: %v", err)
	}
	return path
}

// TestCreateCityDB_Success covers CreateCityDB's happy path: a trivial valid
// CreateDB script and a CreateSchema script that uses the {schema_name} psql
// variable CreateCityDB relies on, run once per configured LOD schema.
func TestCreateCityDB_Success(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "citydb_setup_success_lod2"
	cfg.DB.Schemas.Lod3 = "citydb_setup_success_lod3"
	t.Cleanup(func() {
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.Lod2+` CASCADE`)
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.Lod3+` CASCADE`)
	})

	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	if err := db.CreateCityDB(cfg); err != nil {
		t.Fatalf("CreateCityDB: %v", err)
	}
	if !schemaExists(t, ctx, cfg.DB.Schemas.Lod2) {
		t.Errorf("expected schema %q to exist after CreateCityDB", cfg.DB.Schemas.Lod2)
	}
	if !schemaExists(t, ctx, cfg.DB.Schemas.Lod3) {
		t.Errorf("expected schema %q to exist after CreateCityDB", cfg.DB.Schemas.Lod3)
	}
}

// TestCreateCityDB_CreateDBScriptFailure covers the "failed to create CityDB
// core" wrap: an invalid CreateDB script fails before any schema is touched.
func TestCreateCityDB_CreateDBScriptFailure(t *testing.T) {
	cfg := testConfig("citytabula_dbtest")
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `THIS IS NOT VALID SQL;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	err := db.CreateCityDB(cfg)
	if err == nil {
		t.Fatal("expected an error for an invalid CreateDB script, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create CityDB core") {
		t.Errorf("expected CreateCityDB's own error wrap, got: %v", err)
	}
}

// TestCreateCityDB_CreateSchemaScriptFailure covers the "failed to create
// CityDB schema %s" wrap: CreateDB succeeds, CreateSchema fails.
func TestCreateCityDB_CreateSchemaScriptFailure(t *testing.T) {
	cfg := testConfig("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "citydb_setup_failure_lod2"
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `THIS IS NOT VALID SQL;`)

	err := db.CreateCityDB(cfg)
	if err == nil {
		t.Fatal("expected an error for an invalid CreateSchema script, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create CityDB schema "+cfg.DB.Schemas.Lod2) {
		t.Errorf("expected CreateCityDB's own error wrap naming the schema, got: %v", err)
	}
}
