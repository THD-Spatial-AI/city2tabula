//go:build integration

package onrequest_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// deggendorfTile is a small (~3MB) real CityGML tile from Bavaria's open data.
// One tile keeps this test fast while still exercising the real citydb-tool
// binary and the --bbox flag against real geometry.
const deggendorfTile = "data/lod2/deggendorf/786_5416.gml"

// skipUnlessRealResourcesAvailable skips the test unless this machine has a real
// citydb-tool install and the real TABULA CSVs — both absent by design in most
// CI environments but present on this dev machine. Must be called after
// chdir'ing to the project root (paths below are relative to it).
func skipUnlessRealResourcesAvailable(t *testing.T) (toolPath string) {
	t.Helper()
	toolPath = os.Getenv("CITYDB_TOOL_PATH")
	if toolPath == "" {
		t.Skip("CITYDB_TOOL_PATH not set, skipping real citydb-tool end-to-end test")
	}
	if _, err := os.Stat(filepath.Join(toolPath, "citydb")); err != nil {
		t.Skipf("citydb executable not found at %s, skipping: %v", toolPath, err)
	}
	if _, err := os.Stat(deggendorfTile); err != nil {
		t.Skipf("%s not found, skipping: %v", deggendorfTile, err)
	}
	if _, err := os.Stat("data/tabula/germany.csv"); err != nil {
		t.Skipf("data/tabula/germany.csv not found, skipping: %v", err)
	}
	return toolPath
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

// seedEmptyPylovoTables creates minimal, empty pylovo.res/oth tables so
// -link-pylovo (run as part of RunForRegion) has something to query against —
// with no rows to match, every imported building ends up match_type=2 ("3D
// only"), which is still a real building_link row.
func seedEmptyPylovoTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS public.res CASCADE`,
		`DROP TABLE IF EXISTS public.oth CASCADE`,
		`CREATE TABLE public.res (osm_id TEXT, geom GEOMETRY(MultiPolygon, 3035))`,
		`CREATE TABLE public.oth (osm_id TEXT, geom GEOMETRY(MultiPolygon, 3035))`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seedEmptyPylovoTables (%s): %v", stmt, err)
		}
	}
}

// e2eConfig builds a Config by hand for the given DB/tool settings — it never
// calls config.LoadEnv()/LoadBaseConfig(), so it can never pick up this
// project's real .env (which points at a real local Postgres, not the ephemeral
// test container: see pipelineConfig in internal/process/integration_test.go
// for the same reasoning applied to the pipeline tests).
func e2eConfig(t *testing.T, host, port, dbName, toolPath string) *config.Config {
	t.Helper()
	code, err := config.CountryCode("germany")
	if err != nil {
		t.Fatalf("CountryCode: %v", err)
	}
	srid, srsName, err := config.SRIDForCountry("germany")
	if err != nil {
		t.Fatalf("SRIDForCountry: %v", err)
	}

	cfg := &config.Config{
		Country:     "germany",
		CountryCode: code,
		DB: &config.DBConfig{
			Host:     host,
			Port:     port,
			Name:     dbName,
			User:     testutil.TestUser,
			Password: testutil.TestPassword,
			SSLMode:  "disable",
			Tables: &config.Tables{
				Tabula:        config.Tabula,
				TabulaVariant: config.TabulaVariant,
			},
			Schemas: &config.Schemas{
				Public:      config.PublicSchema,
				CityDB:      config.CityDBSchema,
				CityDBPkg:   config.CityDBPkgSchema,
				Lod2:        config.Lod2Schema,
				Lod3:        config.Lod3Schema,
				Tabula:      config.TabulaSchema,
				City2Tabula: config.City2TabulaSchema,
				Pylvo:       config.PylvoSchemaName, // "public" - matches seedEmptyPylovoTables
			},
		},
		CityDB: &config.CityDB{
			ToolPath:    toolPath,
			SRID:        srid,
			SRSName:     srsName,
			LODLevels:   []int{2, 3},
			ImportLimit: 0,
		},
		City2Tabula: &config.City2TabulaConfig{RoomHeight: "2.5", LinkGridSize: 1000},
		Batch:       &config.BatchConfig{Size: 1000, Threads: 2},
		RetryConfig: config.DefaultRetryConfig(),
	}
	cfg.CityDB.SQLScripts.CreateDB = filepath.Join(toolPath, "3dcitydb", "postgresql", "sql-scripts", "create-db.sql")
	cfg.CityDB.SQLScripts.CreateSchema = filepath.Join(toolPath, "3dcitydb", "postgresql", "sql-scripts", "create-schema.sql")
	return cfg
}

// TestRunForRegion_RealCitydbTool_ImportsAndLinksBuildings drives RunForRegion
// end to end against a real citydb-tool binary and GML tile: bbox-scoped import
// into an already-provisioned database, then extraction and PyLovo linking.
// The bbox is generous since the tile's exact extent isn't known up front —
// this proves the --bbox plumbing works, not tight filtering (unit-tested
// separately in internal/importer).
func TestRunForRegion_RealCitydbTool_ImportsAndLinksBuildings(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	utils.InitLogger()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	projectRoot := filepath.Join(wd, "..", "..") // internal/onrequest -> project root
	t.Chdir(projectRoot)

	toolPath := skipUnlessRealResourcesAvailable(t)

	host, port := testutil.StartPostGISAddr(t)
	cfg := e2eConfig(t, host, port, "onrequest_e2e_test_de", toolPath)

	// Point Data.Lod2 at a temp dir holding just the one small tile, instead of
	// the real (much larger) data/lod2/germany/ directory.
	lod2Dir := t.TempDir()
	copyFile(t, deggendorfTile, filepath.Join(lod2Dir, filepath.Base(deggendorfTile)))
	cfg.Data = &config.DataPaths{
		Lod2:   lod2Dir,
		Lod3:   t.TempDir(), // empty -> skipped
		Tabula: filepath.Join(projectRoot, "data", "tabula") + string(filepath.Separator),
	}

	// Provision the database and schemas up front (mirrors CreateCompleteDatabase's
	// first two steps) so RunForRegion sees an already-existing country and takes
	// its incremental-import branch. ConnectPool must run before CreateCityDB: it's
	// the step that both creates the database (via EnsureDatabase) and enables the
	// PostGIS extension create-db.sql's own script depends on.
	pool, err := db.ConnectPool(cfg)
	if err != nil {
		t.Fatalf("ConnectPool: %v", err)
	}
	t.Cleanup(func() { db.ClosePool(pool) })
	if err := db.CreateCityDB(cfg); err != nil {
		t.Fatalf("CreateCityDB: %v", err)
	}
	if err := db.RunCity2TabulaDBSetup(cfg, pool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	seedEmptyPylovoTables(t, pool)

	// Generous WGS84 box around Deggendorf/Bavaria - comfortably covers the tile.
	bbox := onrequest.Bbox{Xmin: 12.0, Ymin: 48.5, Xmax: 13.5, Ymax: 49.2}

	if err := onrequest.RunForRegion(cfg, bbox, "intersects"); err != nil {
		t.Fatalf("RunForRegion: %v", err)
	}

	count, err := onrequest.CountBuildingLink(context.Background(), pool, cfg, bbox)
	if err != nil {
		t.Fatalf("CountBuildingLink: %v", err)
	}
	if count == 0 {
		t.Error("expected at least one building_link row after a real import of a real GML tile, got 0")
	}

	buildings, err := onrequest.BuildingsByOSMIDs(context.Background(), pool, cfg, []string{"nonexistent-osm-id"})
	if err != nil {
		t.Fatalf("BuildingsByOSMIDs: %v", err)
	}
	if len(buildings) != 0 {
		t.Errorf("expected no match for a nonexistent osm_id, got %d", len(buildings))
	}
}
