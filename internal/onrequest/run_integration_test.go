//go:build integration

package onrequest_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
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

// deggendorfAttribution credits the tile's provider, as the dataset folder the
// importer reads it from must.
const deggendorfAttribution = `{
  "schema_version": 1,
  "dataset_id": "de-bavaria-lod2",
  "provider": "Bayerische Vermessungsverwaltung",
  "dataset": "3D-Gebäudemodelle (LoD2)",
  "licence": "CC-BY-4.0",
  "licence_url": "https://creativecommons.org/licenses/by/4.0/",
  "credit": "Bayerische Vermessungsverwaltung – www.geodaten.bayern.de",
  "changes": "Test import of one tile."
}`

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
		`CREATE TABLE public.res (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
		`CREATE TABLE public.oth (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
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
		City2Tabula: &config.City2TabulaConfig{StoreyHeight: "2.8", LinkGridSize: 1000},
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

	// Point Data.Lod2 at a temp dir holding one dataset folder with just the one
	// small tile, instead of the real (much larger) data/lod2/germany/ directory.
	lod2Dir := t.TempDir()
	datasetDir := filepath.Join(lod2Dir, "deggendorf")
	if err := os.MkdirAll(filepath.Join(datasetDir, "gml"), 0o755); err != nil {
		t.Fatalf("create dataset folder: %v", err)
	}
	copyFile(t, deggendorfTile, filepath.Join(datasetDir, "gml", filepath.Base(deggendorfTile)))
	if err := os.WriteFile(filepath.Join(datasetDir, "attribution.json"), []byte(deggendorfAttribution), 0o644); err != nil {
		t.Fatalf("write attribution.json: %v", err)
	}
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

	var stages []onrequest.Stage
	onStage := func(stage onrequest.Stage) { stages = append(stages, stage) }
	if err := onrequest.RunForRegion(cfg, bbox, "intersects", onStage); err != nil {
		t.Fatalf("RunForRegion: %v", err)
	}
	if !slices.Equal(stages, onrequest.Stages) {
		t.Errorf("reported stages %+v, want each of %+v in order", stages, onrequest.Stages)
	}

	// seedEmptyPylovoTables leaves nothing to match against, so every building
	// links as match_type 2 and coverage is zero even though the import worked.
	count, err := onrequest.CountBuildingLink(context.Background(), pool, cfg, bbox)
	if err != nil {
		t.Fatalf("CountBuildingLink: %v", err)
	}
	if count != 0 {
		t.Errorf("expected zero coverage with no PyLovo buildings to match, got %d", count)
	}

	// The import itself is what this test exercises, so assert on the rows the
	// linker wrote rather than on the matched subset.
	var linkRows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM city2tabula.building_link`,
	).Scan(&linkRows); err != nil {
		t.Fatalf("count building_link rows: %v", err)
	}
	if linkRows == 0 {
		t.Error("expected at least one building_link row after a real import of a real GML tile, got 0")
	}

	// Every extracted building carries the dataset folder's dataset_id, read
	// from the lineage the import gave its feature.
	var withDataset, total int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE dataset_id = 'de-bavaria-lod2'), count(*) FROM city2tabula.lod2_building`,
	).Scan(&withDataset, &total); err != nil {
		t.Fatalf("count lod2_building dataset_id: %v", err)
	}
	if total == 0 || withDataset != total {
		t.Errorf("expected every one of %d buildings to have dataset_id de-bavaria-lod2, got %d", total, withDataset)
	}

	buildings, err := onrequest.BuildingsByOSMIDs(context.Background(), pool, cfg, []string{"nonexistent-osm-id"})
	if err != nil {
		t.Fatalf("BuildingsByOSMIDs: %v", err)
	}
	if len(buildings) != 0 {
		t.Errorf("expected no match for a nonexistent osm_id, got %d", len(buildings))
	}
}

// tileBbox returns the WGS84 envelope of an EPSG:25832 rectangle, so test boxes
// can be placed exactly on deggendorfTile, which covers x 786-788 km, y 5416-5418 km.
func tileBbox(t *testing.T, pool *pgxpool.Pool, xmin, ymin, xmax, ymax float64) onrequest.Bbox {
	t.Helper()
	var b onrequest.Bbox
	if err := pool.QueryRow(context.Background(), `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, 25832), 4326) AS e) x`,
		xmin, ymin, xmax, ymax,
	).Scan(&b.Xmin, &b.Ymin, &b.Xmax, &b.Ymax); err != nil {
		t.Fatalf("transform bbox: %v", err)
	}
	return b
}

// buildingCounts returns the LOD2 Building features and extracted buildings, and
// how many object ids either holds more than once.
func buildingCounts(t *testing.T, pool *pgxpool.Pool) (features, buildings, repeated int) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM lod2.feature WHERE objectclass_id = 901),
		       (SELECT count(*) FROM city2tabula.lod2_building),
		       (SELECT count(*) FROM (SELECT objectid FROM lod2.feature WHERE objectclass_id = 901
		                              GROUP BY objectid HAVING count(*) > 1) f)
		     + (SELECT count(*) FROM (SELECT object_id FROM city2tabula.lod2_building
		                              GROUP BY object_id HAVING count(*) > 1) b)`,
	).Scan(&features, &buildings, &repeated); err != nil {
		t.Fatalf("count buildings: %v", err)
	}
	return features, buildings, repeated
}

// TestRunForRegion_RealCitydbTool_OverlapsImportOnce pins that runs over
// overlapping bboxes, and a run after a dataset folder gains a file holding
// buildings already imported, store each building once. The second file is a
// copy of the first tile, the extreme case of two tiles sharing buildings.
func TestRunForRegion_RealCitydbTool_OverlapsImportOnce(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	projectRoot := filepath.Join(wd, "..", "..")
	t.Chdir(projectRoot)
	toolPath := skipUnlessRealResourcesAvailable(t)

	host, port := testutil.StartPostGISAddr(t)
	cfg := e2eConfig(t, host, port, "onrequest_overlap_test_de", toolPath)

	lod2Dir := t.TempDir()
	gmlDir := filepath.Join(lod2Dir, "deggendorf", "gml")
	if err := os.MkdirAll(gmlDir, 0o755); err != nil {
		t.Fatalf("create dataset folder: %v", err)
	}
	copyFile(t, deggendorfTile, filepath.Join(gmlDir, filepath.Base(deggendorfTile)))
	if err := os.WriteFile(filepath.Join(lod2Dir, "deggendorf", "attribution.json"), []byte(deggendorfAttribution), 0o644); err != nil {
		t.Fatalf("write attribution.json: %v", err)
	}
	cfg.Data = &config.DataPaths{
		Lod2:   lod2Dir,
		Lod3:   t.TempDir(),
		Tabula: filepath.Join(projectRoot, "data", "tabula") + string(filepath.Separator),
	}

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

	run := func(name string, bbox onrequest.Bbox) (features, buildings int) {
		t.Helper()
		if err := onrequest.RunForRegion(cfg, bbox, "intersects", func(onrequest.Stage) {}); err != nil {
			t.Fatalf("%s: RunForRegion: %v", name, err)
		}
		features, buildings, repeated := buildingCounts(t, pool)
		if repeated != 0 {
			t.Errorf("%s: %d object ids stored more than once (%d features, %d buildings)", name, repeated, features, buildings)
		}
		return features, buildings
	}

	west := tileBbox(t, pool, 786000, 5416000, 787200, 5418000)
	east := tileBbox(t, pool, 786800, 5416000, 788000, 5418000)
	whole := tileBbox(t, pool, 786000, 5416000, 788000, 5418000)

	westFeatures, _ := run("west bbox", west)
	if westFeatures == 0 {
		t.Fatal("west bbox imported no buildings; the box misses the tile")
	}
	features, buildings := run("overlapping east bbox", east)
	if features <= westFeatures {
		t.Errorf("east bbox added no buildings: %d features after, %d before", features, westFeatures)
	}

	copyFile(t, deggendorfTile, filepath.Join(gmlDir, "copy_of_"+filepath.Base(deggendorfTile)))
	againFeatures, againBuildings := run("whole tile after adding a copy", whole)
	if againFeatures != features || againBuildings != buildings {
		t.Errorf("re-import after adding a copy changed the counts: features %d -> %d, buildings %d -> %d",
			features, againFeatures, buildings, againBuildings)
	}
}
