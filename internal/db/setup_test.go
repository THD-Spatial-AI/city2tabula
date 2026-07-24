//go:build integration

package db_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// projectRoot returns the absolute path to the repository root so tests that
// load SQL files from disk (via config.LoadSQLScripts's relative paths) can
// chdir to the right location, mirroring internal/process's own helper.
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestDropCityDBSchemas_DropsAllFourSchemas(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	cfg.DB.Schemas.CityDB = "citydb_drop_test"
	cfg.DB.Schemas.CityDBPkg = "citydb_pkg_drop_test"

	schemas := []string{cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3, cfg.DB.Schemas.CityDB, cfg.DB.Schemas.CityDBPkg}
	if err := db.CreateSchemas(testPool, schemas); err != nil {
		t.Fatalf("failed to seed schemas: %v", err)
	}

	if err := db.DropCityDBSchemas(cfg, testPool); err != nil {
		t.Fatalf("DropCityDBSchemas: %v", err)
	}
	for _, s := range schemas {
		if schemaExists(t, ctx, s) {
			t.Errorf("expected schema %q to be gone after DropCityDBSchemas", s)
		}
	}
}

func TestDropAllSchemas_DropsBothCityDBAndCity2TabulaSchemas(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	cfg.DB.Schemas.CityDB = "citydb_dropall_test"
	cfg.DB.Schemas.CityDBPkg = "citydb_pkg_dropall_test"
	cfg.DB.Schemas.City2Tabula = "city2tabula_dropall_test"
	cfg.DB.Schemas.Tabula = "tabula_dropall_test"

	allSchemas := []string{
		cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3, cfg.DB.Schemas.CityDB, cfg.DB.Schemas.CityDBPkg,
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Tabula,
	}
	if err := db.CreateSchemas(testPool, allSchemas); err != nil {
		t.Fatalf("failed to seed schemas: %v", err)
	}

	if err := db.DropAllSchemas(cfg, testPool); err != nil {
		t.Fatalf("DropAllSchemas: %v", err)
	}
	for _, s := range allSchemas {
		if schemaExists(t, ctx, s) {
			t.Errorf("expected schema %q to be gone after DropAllSchemas", s)
		}
	}
}

// TestRunCity2TabulaDBSetup_Success covers RunCity2TabulaDBSetup, setupMainDB,
// and setupSupplementaryDB's success paths together: real SQL DDL from
// sql/schema/main/ and sql/schema/supplementary/, run from the project root.
func TestRunCity2TabulaDBSetup_Success(t *testing.T) {
	t.Chdir(projectRoot())
	ctx := context.Background()
	cfg := testConfig("citytabula_dbtest")
	cfg.DB.Schemas.City2Tabula = "city2tabula_setup_test"
	cfg.DB.Schemas.Tabula = "tabula_setup_test"
	cfg.DB.Tables = &config.Tables{Tabula: "tabula", TabulaVariant: "tabula_variant"}
	cfg.City2Tabula = &config.City2TabulaConfig{RoomHeight: "2.5"}
	cfg.RetryConfig = config.DefaultRetryConfig()
	t.Cleanup(func() {
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.City2Tabula+` CASCADE`)
		testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+cfg.DB.Schemas.Tabula+` CASCADE`)
	})

	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	var exists bool
	if err := testPool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = 'lod2_building')`,
		cfg.DB.Schemas.City2Tabula,
	).Scan(&exists); err != nil {
		t.Fatalf("failed to check for lod2_building table: %v", err)
	}
	if !exists {
		t.Error("expected sql/schema/main/'s lod2_building table to exist after setup")
	}
}

// TestSetupMainDB_LoadSQLScriptsFailurePropagates and its supplementary
// counterpart drive the LoadSQLScripts error-wrap in each of RunCity2TabulaDBSetup's
// two internal steps - filesystem-only, no DB touched (the failure happens
// before any SQL runs).
func TestRunCity2TabulaDBSetup_LoadSQLScriptsFailurePropagates(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := testConfig("citytabula_dbtest")

	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err == nil {
		t.Error("expected RunCity2TabulaDBSetup to propagate a LoadSQLScripts failure, got nil")
	}
}
