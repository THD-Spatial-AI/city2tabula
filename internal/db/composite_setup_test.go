//go:build integration

package db_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// fullCfg extends testConfig with everything RunCity2TabulaDBSetup's real SQL
// scripts need (config.GetSQLParameters dereferences DB.Tables and
// City2Tabula unconditionally), so callers only set what's specific to their
// scenario.
func fullCfg(dbName string) *config.Config {
	cfg := testConfig(dbName)
	cfg.DB.Tables = &config.Tables{Tabula: "tabula", TabulaVariant: "tabula_variant"}
	cfg.City2Tabula = &config.City2TabulaConfig{RoomHeight: "2.5"}
	cfg.RetryConfig = config.DefaultRetryConfig()
	return cfg
}

// writeFakeCityDBExecutable writes a real (but trivial) shell script named
// "citydb" that just exits with exitCode, standing in for the actual CityDB
// Java CLI tool, which isn't available in the test environment - same
// technique used in internal/importer's own tests. Returns the containing
// directory (ImportCityDBData joins ToolPath + "citydb").
func writeFakeCityDBExecutable(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "citydb")
	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake citydb executable: %v", err)
	}
	return dir
}

func dropSchemasOnCleanup(t *testing.T, ctx context.Context, schemas ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, s := range schemas {
			testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+s+` CASCADE`)
		}
	})
}

// --- CreateCompleteDatabase ---

func TestCreateCompleteDatabase_CreateCityDBFailure(t *testing.T) {
	cfg := fullCfg("citytabula_dbtest")
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `THIS IS NOT VALID SQL;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	err := db.CreateCompleteDatabase(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when CreateCityDB fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to setup CityDB infrastructure") {
		t.Errorf("expected CreateCompleteDatabase's own error wrap, got: %v", err)
	}
}

func TestCreateCompleteDatabase_RunCity2TabulaDBSetupFailure(t *testing.T) {
	ctx := context.Background()
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "ccd_c2t_fail_lod2"
	cfg.DB.Schemas.Lod3 = "ccd_c2t_fail_lod3"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3)
	// Absolute paths, unaffected by the chdir below.
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	t.Chdir(t.TempDir()) // no sql/ tree here -> LoadSQLScripts fails

	err := db.CreateCompleteDatabase(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when RunCity2TabulaDBSetup fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create City2TABULA schemas") {
		t.Errorf("expected CreateCompleteDatabase's own error wrap, got: %v", err)
	}
}

// TestCreateCompleteDatabase_ImportAllDataFailure drives CreateCompleteDatabase's
// third step failing: the first two steps succeed for real (fake harmless
// CityDB scripts, real sql/schema/ DDL from the project root), but
// ImportAllData -> ImportSupplementaryData -> ImportTabulaData fails because
// there's no real TABULA CSV at the configured path. A full success run isn't
// reachable in this test environment: the real CSV is a 200+ column file this
// repo doesn't ship (sourced from the TABULA project, not generated here).
func TestCreateCompleteDatabase_ImportAllDataFailure(t *testing.T) {
	ctx := context.Background()
	t.Chdir(projectRoot())
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "ccd_import_fail_lod2"
	cfg.DB.Schemas.Lod3 = "ccd_import_fail_lod3"
	cfg.DB.Schemas.City2Tabula = "ccd_import_fail_c2t"
	cfg.DB.Schemas.Tabula = "ccd_import_fail_tabula"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Tabula)
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)
	cfg.Country = "germany"
	cfg.Data = &config.DataPaths{Tabula: t.TempDir() + string(filepath.Separator)} // no matching CSV in here

	err := db.CreateCompleteDatabase(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when ImportAllData fails (no real TABULA CSV), got nil")
	}
	if !strings.Contains(err.Error(), "failed to import data") {
		t.Errorf("expected CreateCompleteDatabase's own error wrap, got: %v", err)
	}
}

// --- ResetCompleteDatabase ---

// TestResetCompleteDatabase_CreateCompleteDatabaseFailure covers
// ResetCompleteDatabase's own error wrap. Its first step (DropAllSchemas)
// never returns an error (every failure inside it is warn-only), so the only
// reachable failure is step 2.
func TestResetCompleteDatabase_CreateCompleteDatabaseFailure(t *testing.T) {
	cfg := fullCfg("citytabula_dbtest")
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `THIS IS NOT VALID SQL;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	err := db.ResetCompleteDatabase(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when the recreate step fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to recreate database") {
		t.Errorf("expected ResetCompleteDatabase's own error wrap, got: %v", err)
	}
}

// --- ResetCityDBOnly ---

func TestResetCityDBOnly_CreateCityDBFailure(t *testing.T) {
	cfg := fullCfg("citytabula_dbtest")
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `THIS IS NOT VALID SQL;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)

	err := db.ResetCityDBOnly(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when CreateCityDB fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to recreate CityDB") {
		t.Errorf("expected ResetCityDBOnly's own error wrap, got: %v", err)
	}
}

func TestResetCityDBOnly_ImportCityDBDataFailure(t *testing.T) {
	ctx := context.Background()
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "rcdo_import_fail_lod2"
	cfg.DB.Schemas.Lod3 = "rcdo_import_fail_lod3"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3)
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)
	cfg.CityDB.ToolPath = writeFakeCityDBExecutable(t, 1) // fails -help

	err := db.ResetCityDBOnly(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when ImportCityDBData fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to import CityDB data") {
		t.Errorf("expected ResetCityDBOnly's own error wrap, got: %v", err)
	}
}

// TestResetCityDBOnly_Success is the one full end-to-end success case reachable
// without a real TABULA CSV or the real CityDB tool: ResetCityDBOnly never
// calls ImportSupplementaryData, and importCityDBFiles treats a missing LOD
// data directory as an optional skip (warn, not fail) rather than an error -
// so pointing Data.Lod2/Lod3 at paths that don't exist lets the fake citydb
// executable's -help check succeed and the whole call return nil.
func TestResetCityDBOnly_Success(t *testing.T) {
	ctx := context.Background()
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Lod2 = "rcdo_success_lod2"
	cfg.DB.Schemas.Lod3 = "rcdo_success_lod3"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3)
	cfg.CityDB.SQLScripts.CreateDB = writeSQLFixture(t, `SELECT 1;`)
	cfg.CityDB.SQLScripts.CreateSchema = writeSQLFixture(t, `CREATE SCHEMA :"schema_name";`)
	cfg.CityDB.ToolPath = writeFakeCityDBExecutable(t, 0) // succeeds -help
	cfg.Data = &config.DataPaths{Lod2: "/nonexistent/lod2", Lod3: "/nonexistent/lod3"}

	if err := db.ResetCityDBOnly(cfg, testPool); err != nil {
		t.Fatalf("ResetCityDBOnly: %v", err)
	}
	if !schemaExists(t, ctx, cfg.DB.Schemas.Lod2) {
		t.Errorf("expected schema %q to exist after ResetCityDBOnly", cfg.DB.Schemas.Lod2)
	}
}

// --- ImportAllData ---

// TestImportAllData_ImportSupplementaryDataFailure covers ImportAllData's
// first error wrap directly: no real TABULA CSV at the configured path.
func TestImportAllData_ImportSupplementaryDataFailure(t *testing.T) {
	cfg := fullCfg("citytabula_dbtest")
	cfg.Country = "germany"
	cfg.Data = &config.DataPaths{Tabula: t.TempDir() + string(filepath.Separator)}

	err := db.ImportAllData(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when ImportSupplementaryData fails (no real TABULA CSV), got nil")
	}
	if !strings.Contains(err.Error(), "failed to import supplementary data") {
		t.Errorf("expected ImportAllData's own error wrap, got: %v", err)
	}
}

// --- ResetCity2TabulaSchemas ---

// TestResetCity2TabulaSchemas_ImportSupplementaryDataFailure covers
// ResetCity2TabulaSchemas end to end up to its one unreachable-without-a-real-
// CSV step: the drop loop and RunCity2TabulaDBSetup both succeed for real
// (project root, real DDL), and the final importer.ImportSupplementaryData
// call fails the same way as ImportAllData's - no real TABULA CSV shipped in
// this repo.
func TestResetCity2TabulaSchemas_ImportSupplementaryDataFailure(t *testing.T) {
	ctx := context.Background()
	t.Chdir(projectRoot())
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.City2Tabula = "rc2t_fail_c2t"
	cfg.DB.Schemas.Tabula = "rc2t_fail_tabula"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Tabula)
	cfg.Country = "germany"
	cfg.Data = &config.DataPaths{Tabula: t.TempDir() + string(filepath.Separator)}

	err := db.ResetCity2TabulaSchemas(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when ImportSupplementaryData fails (no real TABULA CSV), got nil")
	}
	if !strings.Contains(err.Error(), "No such file or directory") {
		t.Errorf("expected the underlying missing-CSV error to propagate unwrapped, got: %v", err)
	}
	// Then: the schemas from the successful RunCity2TabulaDBSetup step exist,
	// proving the function got past both the drop loop and the rebuild before
	// failing on the CSV import.
	if !schemaExists(t, ctx, cfg.DB.Schemas.City2Tabula) {
		t.Error("expected city2tabula schema to exist (RunCity2TabulaDBSetup ran before the failure)")
	}
}
