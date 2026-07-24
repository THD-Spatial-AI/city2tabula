//go:build integration

package db_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// setupMinimalTabulaTable pre-creates a minimal stand-in tabula.tabula table
// - just the 17 columns sql/scripts/supplementary/01_extract_tabula_attributes.sql
// actually SELECTs, not the real 216-column TABULA project format this repo
// doesn't ship. Only safe for callers that DON'T go on to run
// sql/schema/supplementary/01_create_tabula_tables.sql afterwards: that
// script unconditionally DROP TABLEs tabula.tabula before its own CREATE
// TABLE IF NOT EXISTS, wiping this fixture out and replacing it with the
// real 216-column shape (RunCity2TabulaDBSetup runs that script, so anything
// that calls it - CreateCompleteDatabase, ResetCompleteDatabase,
// ResetCity2TabulaSchemas - can't use this trick for a full success test;
// see internal/importer's supplementary_integration_test.go, which sidesteps
// this by running RunCity2TabulaDBSetup itself first and only overriding the
// table afterwards). ImportAllData never touches this DDL at all, so it's
// the one composite entry point this fixture works for directly.
//
// The schema must be the literal "tabula", not cfg.DB.Schemas.Tabula:
// importer.ImportCsvWithPsql hardcodes "\copy tabula.tabula FROM ..." rather
// than substituting the configured schema/table name (unlike the extraction
// SQL, which is correctly parameterized) - a real inconsistency, flagged
// separately, not something to route around silently here.
func setupMinimalTabulaTable(t *testing.T, ctx context.Context) {
	t.Helper()
	t.Cleanup(func() { testPool.Exec(ctx, `DROP SCHEMA IF EXISTS tabula CASCADE`) })
	if _, err := testPool.Exec(ctx, `DROP SCHEMA IF EXISTS tabula CASCADE`); err != nil {
		t.Fatalf("failed to reset tabula schema: %v", err)
	}
	if _, err := testPool.Exec(ctx, `CREATE SCHEMA tabula`); err != nil {
		t.Fatalf("failed to create tabula schema: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		CREATE TABLE tabula.tabula (
			id INTEGER,
			"Code_BuildingVariant" TEXT,
			"Number_BuildingVariant" INTEGER,
			"Year1_Building" INTEGER,
			"Year2_Building" INTEGER,
			"V_C" DOUBLE PRECISION,
			"A_C_National" DOUBLE PRECISION,
			"n_Storey" INTEGER,
			"Code_ComplexFootprint" TEXT,
			"Code_AttachedNeighbours" TEXT,
			"Code_ComplexRoof" TEXT,
			"A_Roof_1" DOUBLE PRECISION,
			"A_Roof_2" DOUBLE PRECISION,
			"A_Wall_1" DOUBLE PRECISION,
			"A_Wall_2" DOUBLE PRECISION,
			"A_Wall_3" DOUBLE PRECISION,
			"A_C_ExtDim" DOUBLE PRECISION,
			"Code_BuildingSizeClass" TEXT
		);`,
	); err != nil {
		t.Fatalf("failed to create minimal tabula.tabula fixture: %v", err)
	}
}

// writeMinimalTabulaCSV writes one data row matching setupMinimalTabulaTable's
// column order exactly - empty fields become NULL under COPY ... CSV, which
// COALESCE(..., 0) in 01_extract_tabula_attributes.sql then zeroes out.
func writeMinimalTabulaCSV(t *testing.T, dir, country string) {
	t.Helper()
	header := `id,Code_BuildingVariant,Number_BuildingVariant,Year1_Building,Year2_Building,V_C,A_C_National,n_Storey,Code_ComplexFootprint,Code_AttachedNeighbours,Code_ComplexRoof,A_Roof_1,A_Roof_2,A_Wall_1,A_Wall_2,A_Wall_3,A_C_ExtDim,Code_BuildingSizeClass`
	row := `1,TEST.VARIANT.001,1,1990,2000,500.0,150.0,3,Regular,B_Alone,Simple,60.0,,40.0,40.0,,145.0,SFH`
	csv := header + "\n" + row + "\n"
	path := filepath.Join(dir, country+".csv")
	if err := os.WriteFile(path, []byte(csv), 0644); err != nil {
		t.Fatalf("failed to write CSV fixture: %v", err)
	}
}

// TestImportAllData_Success drives ImportAllData's own success return
// directly: the minimal tabula.tabula fixture above satisfies
// ImportSupplementaryData for real, and ImportCityDBData's LOD2/LOD3 import
// is skipped (warn, not fail) by pointing Data.Lod2/Lod3 at nonexistent
// directories - so every step runs for real against Postgres and returns
// nil.
func TestImportAllData_Success(t *testing.T) {
	ctx := context.Background()
	t.Chdir(projectRoot()) // ImportSupplementaryData loads sql/scripts/supplementary/ relative to cwd
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Tabula = "tabula" // must match ImportCsvWithPsql's hardcoded "tabula.tabula" target
	cfg.DB.Schemas.City2Tabula = "iad_success_c2t"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.City2Tabula)
	cfg.CityDB.ToolPath = writeFakeCityDBExecutable(t, 0)
	cfg.Country = "germany"

	dataDir := t.TempDir()
	writeMinimalTabulaCSV(t, dataDir, cfg.Country)
	cfg.Data = &config.DataPaths{
		Tabula: dataDir + string(filepath.Separator),
		Lod2:   "/nonexistent/lod2",
		Lod3:   "/nonexistent/lod3",
	}

	if _, err := testPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+cfg.DB.Schemas.City2Tabula); err != nil {
		t.Fatalf("failed to seed city2tabula schema: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		CREATE TABLE `+cfg.DB.Schemas.City2Tabula+`.tabula_variant (
			id SERIAL PRIMARY KEY,
			tabula_variant_code_id INTEGER NOT NULL UNIQUE,
			tabula_variant_code TEXT NOT NULL,
			max_volume DOUBLE PRECISION,
			total_area DOUBLE PRECISION,
			construction_year_1 INTEGER,
			construction_year_2 INTEGER,
			footprint_area DOUBLE PRECISION,
			number_of_storeys INTEGER,
			footprint_complexity INTEGER CHECK (footprint_complexity IN (-1, 0, 1, 2)),
			attached_neighbour_class INTEGER CHECK (attached_neighbour_class IN (-1, 0, 1, 2)),
			roof_complexity INTEGER CHECK (roof_complexity IN (-1, 0, 1, 2)),
			area_total_roof DOUBLE PRECISION,
			area_total_wall DOUBLE PRECISION,
			area_total_floor DOUBLE PRECISION,
			building_size_class INTEGER CHECK (building_size_class IN (-1, 0, 1, 2, 3))
		);`,
	); err != nil {
		t.Fatalf("failed to seed tabula_variant table: %v", err)
	}
	setupMinimalTabulaTable(t, ctx)

	if err := db.ImportAllData(cfg, testPool); err != nil {
		t.Fatalf("ImportAllData: %v", err)
	}

	// RunJobQueue never returns an error for a failed task (see the
	// coverage-push PR description), so a nil error here doesn't by itself
	// prove the extraction SQL ran - check the resulting row for real.
	var code string
	if err := testPool.QueryRow(ctx,
		`SELECT tabula_variant_code FROM `+cfg.DB.Schemas.City2Tabula+`.tabula_variant WHERE tabula_variant_code_id = 1`,
	).Scan(&code); err != nil {
		t.Fatalf("failed to read the inserted tabula_variant row: %v", err)
	}
	if code != "TEST.VARIANT.001" {
		t.Errorf("tabula_variant_code = %q, want %q", code, "TEST.VARIANT.001")
	}
}

// TestImportAllData_ImportCityDBDataFailure covers ImportAllData's second
// error wrap: ImportSupplementaryData succeeds for real (minimal fixture),
// then ImportCityDBData fails because the fake citydb executable rejects
// even -help.
func TestImportAllData_ImportCityDBDataFailure(t *testing.T) {
	ctx := context.Background()
	t.Chdir(projectRoot())
	cfg := fullCfg("citytabula_dbtest")
	cfg.DB.Schemas.Tabula = "tabula" // must match ImportCsvWithPsql's hardcoded "tabula.tabula" target
	cfg.DB.Schemas.City2Tabula = "iad_citydb_fail_c2t"
	dropSchemasOnCleanup(t, ctx, cfg.DB.Schemas.City2Tabula)
	cfg.CityDB.ToolPath = writeFakeCityDBExecutable(t, 1) // fails -help
	cfg.Country = "germany"

	dataDir := t.TempDir()
	writeMinimalTabulaCSV(t, dataDir, cfg.Country)
	cfg.Data = &config.DataPaths{Tabula: dataDir + string(filepath.Separator)}

	if _, err := testPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+cfg.DB.Schemas.City2Tabula); err != nil {
		t.Fatalf("failed to seed city2tabula schema: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		CREATE TABLE `+cfg.DB.Schemas.City2Tabula+`.tabula_variant (
			id SERIAL PRIMARY KEY,
			tabula_variant_code_id INTEGER NOT NULL UNIQUE,
			tabula_variant_code TEXT NOT NULL,
			max_volume DOUBLE PRECISION,
			total_area DOUBLE PRECISION,
			construction_year_1 INTEGER,
			construction_year_2 INTEGER,
			footprint_area DOUBLE PRECISION,
			number_of_storeys INTEGER,
			footprint_complexity INTEGER CHECK (footprint_complexity IN (-1, 0, 1, 2)),
			attached_neighbour_class INTEGER CHECK (attached_neighbour_class IN (-1, 0, 1, 2)),
			roof_complexity INTEGER CHECK (roof_complexity IN (-1, 0, 1, 2)),
			area_total_roof DOUBLE PRECISION,
			area_total_wall DOUBLE PRECISION,
			area_total_floor DOUBLE PRECISION,
			building_size_class INTEGER CHECK (building_size_class IN (-1, 0, 1, 2, 3))
		);`,
	); err != nil {
		t.Fatalf("failed to seed tabula_variant table: %v", err)
	}
	setupMinimalTabulaTable(t, ctx)

	err := db.ImportAllData(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when ImportCityDBData fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to import CityDB data") {
		t.Errorf("expected ImportAllData's own error wrap, got: %v", err)
	}
}
