//go:build integration

package process_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// testPool is the shared database connection for all integration tests and benchmarks.
// Initialised once in TestMain to avoid starting a container per test.
var testPool *pgxpool.Pool

// testConnStr holds the DSN for psql-based seed execution (COPY FROM stdin requires psql).
var testConnStr string

// testContainer is the running PostGIS container, exposed so tests that need pg_dump/
// pg_restore can exec them inside it — guarantees a version match with the server
// regardless of what (if anything) is installed on the host running `go test`.
var testContainer testcontainers.Container

func TestMain(m *testing.M) {
	// SQL script paths in config are relative to the project root.
	// Tests run from the package directory (internal/process/), so we go up two levels.
	if err := os.Chdir("../.."); err != nil {
		log.Fatalf("failed to change to project root: %v", err)
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "postgis/postgis:17-3.4",
		Env: map[string]string{
			"POSTGRES_DB":       "city2tabula_test",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").AsRegexp(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start PostGIS container: %v", err)
	}
	defer container.Terminate(ctx)
	testContainer = container

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get container port: %v", err)
	}

	testConnStr = fmt.Sprintf("postgres://test:test@%s:%s/city2tabula_test?sslmode=disable", host, port.Port())

	testPool, err = pgxpool.New(ctx, testConnStr+"&jit=off")
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}
	defer testPool.Close()

	// PostGIS images do extra initialization after the "ready" log line.
	// Ping until PostgreSQL is truly accepting connections (up to 15 seconds).
	for i := 0; i < 30; i++ {
		if err := testPool.Ping(ctx); err == nil {
			break
		}
		if i == 29 {
			log.Fatalf("PostgreSQL not ready after 15 seconds")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Enable PostGIS extensions required by the pipeline.
	exts := []string{
		"CREATE EXTENSION IF NOT EXISTS postgis",
		"CREATE EXTENSION IF NOT EXISTS postgis_sfcgal",
	}
	for _, ext := range exts {
		if _, err := testPool.Exec(ctx, ext); err != nil {
			log.Printf("warning: could not enable extension (%s): %v", ext, err)
		}
	}

	os.Exit(m.Run())
}

// pipelineTestCase holds per-country configuration for a pipeline integration test.
type pipelineTestCase struct {
	country string
	srid    string
	seeds   []string // SQL files seeded in order via psql
}

// pipelineConfig builds a Config for the given test case.
// It bypasses LoadConfig() / .env so no environment setup is required.
func pipelineConfig(tc pipelineTestCase) *config.Config {
	code, _ := config.CountryCode(tc.country)
	return &config.Config{
		Country:     tc.country,
		CountryCode: code,
		DB: &config.DBConfig{
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
			},
		},
		CityDB: &config.CityDB{
			SRID:      tc.srid,
			LODLevels: []int{2}, // only LOD2 seed data is available in testdata/
		},
		City2Tabula: &config.City2TabulaConfig{
			RoomHeight:   "2.5",
			StoreyHeight: "2.8",
		},
		Batch: &config.BatchConfig{
			Size:    100,
			Threads: 2,
		},
		RetryConfig: config.DefaultRetryConfig(),
	}
}

// resetSchemas drops the lod2 schema and truncates all city2tabula output tables
// so each country test starts from a clean state within the shared container.
// City2tabula tables may not exist on the first call (before RunCity2TabulaDBSetup
// has run), so truncation is wrapped in a DO block that silently skips missing tables.
func resetSchemas(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if _, err := testPool.Exec(ctx, `DROP SCHEMA IF EXISTS lod2 CASCADE`); err != nil {
		t.Fatalf("resetSchemas: drop lod2: %v", err)
	}

	_, err := testPool.Exec(ctx, `
		DO $$ BEGIN
			TRUNCATE city2tabula.lod2_building         CASCADE;
			TRUNCATE city2tabula.lod2_building_part    CASCADE;
			TRUNCATE city2tabula.lod2_child_feature            CASCADE;
			TRUNCATE city2tabula.lod2_surface_raw    CASCADE;
			TRUNCATE city2tabula.lod2_surface                  CASCADE;
			TRUNCATE city2tabula.lod2_child_feature_geom_dump  CASCADE;
			TRUNCATE city2tabula.tabula_variant                CASCADE;
		EXCEPTION WHEN undefined_table OR invalid_schema_name THEN
			NULL; -- schema/tables not yet created, nothing to truncate
		END $$;
	`)
	if err != nil {
		t.Fatalf("resetSchemas: truncate city2tabula: %v", err)
	}
}

// seedDB executes one or more SQL files via psql, then assigns the seeded
// features a dataset, as an import from a dataset folder would.
// pg_dump files use COPY FROM stdin which pool.Exec() cannot handle —
// psql processes the COPY protocol correctly.
func seedDB(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range append(paths, testDatasetSeed) {
		// ON_ERROR_STOP=1 makes psql exit non-zero on any SQL error so we catch failures.
		cmd := exec.Command("psql", testConnStr, "-v", "ON_ERROR_STOP=1", "-f", path)
		cmd.Env = append(os.Environ(), "PGPASSWORD=test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to seed %s: %v\npsql output:\n%s", path, err, string(out))
		}
	}
}

// testDatasetSeed gives seeded features the lineage "test-dataset" and inserts
// that dataset's dataset_attribution row.
const testDatasetSeed = "testdata/assign_test_dataset.sql"

// runPipelineTest is the shared test driver for all country/LOD pipeline tests.
// It resets schemas, seeds the database, runs feature extraction, and asserts results.
func runPipelineTest(t *testing.T, tc pipelineTestCase) {
	t.Helper()
	ctx := context.Background()

	// Skip if any seed file is missing — CI only runs tests for committed seeds.
	for _, seed := range tc.seeds {
		if _, err := os.Stat(seed); os.IsNotExist(err) {
			t.Skipf("seed file not found, skipping: %s", seed)
		}
	}

	resetSchemas(t)

	cfg := pipelineConfig(tc)

	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	seedDB(t, tc.seeds...)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var buildingCount int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_building",
	).Scan(&buildingCount); err != nil {
		t.Fatalf("failed to query building count: %v", err)
	}
	if buildingCount == 0 {
		t.Error("expected buildings in city2tabula.lod2_building, got 0")
	}

	var labeledCount int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_building WHERE tabula_variant_code IS NOT NULL",
	).Scan(&labeledCount); err != nil {
		t.Fatalf("failed to query labeled count: %v", err)
	}
	if labeledCount == 0 {
		t.Error("expected at least 1 building to have a TABULA variant code assigned")
	}

	// Verify script 04 populated object_id on INSERT (no retroactive backfill needed).
	var missingObjectID int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_building WHERE object_id IS NULL",
	).Scan(&missingObjectID); err != nil {
		t.Fatalf("failed to query object_id coverage: %v", err)
	}
	if missingObjectID > 0 {
		t.Errorf("script 04 failed: %d buildings have no object_id populated", missingObjectID)
	}

	// Verify objectids propagated through all surface layers.
	var missingSurfaceObjectIDs int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_surface_raw WHERE building_object_id IS NULL OR surface_object_id IS NULL",
	).Scan(&missingSurfaceObjectIDs); err != nil {
		t.Fatalf("failed to query surface object_id coverage: %v", err)
	}
	if missingSurfaceObjectIDs > 0 {
		t.Errorf("scripts 01–03 failed: %d surface rows are missing building_object_id or surface_object_id", missingSurfaceObjectIDs)
	}

	// Regression (#115): 3DBAG ships every building at LoD 1.2/1.3/2.2. Script 01
	// must attach only the requested LoD's surface set via the `boundary` property,
	// not every geometry that intersects the solid, so each solid (the Building or
	// one of its BuildingParts) has exactly one GroundSurface.
	var multiGroundSolids int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
			SELECT owner_feature_id FROM city2tabula.lod2_surface_raw
			WHERE classname = 'GroundSurface'
			GROUP BY owner_feature_id HAVING COUNT(*) <> 1
		) x`,
	).Scan(&multiGroundSolids); err != nil {
		t.Fatalf("failed to query ground-surface count: %v", err)
	}
	if multiGroundSolids > 0 {
		t.Errorf("script 01 attached more than one LoD representation: %d solids have != 1 GroundSurface", multiGroundSolids)
	}

	// One row per CityGML Building that owns a solid itself or through its parts,
	// keyed by the Building's own object_id, never a part's.
	var wantBuildings, partKeyed int
	if err := testPool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM lod2.feature b
			 WHERE b.objectclass_id = 901
			   AND EXISTS (
			       SELECT 1 FROM lod2.property s
			       WHERE s.name = 'lod2Solid'
			         AND (s.feature_id = b.id OR s.feature_id IN (
			             SELECT val_feature_id FROM lod2.property
			             WHERE feature_id = b.id AND name = 'buildingPart')))),
			(SELECT COUNT(*) FROM city2tabula.lod2_building lb
			 JOIN lod2.feature f ON f.id = lb.building_feature_id
			 WHERE f.objectclass_id <> 901 OR f.objectid IS DISTINCT FROM lb.object_id)
	`).Scan(&wantBuildings, &partKeyed); err != nil {
		t.Fatalf("failed to query building identity: %v", err)
	}
	if buildingCount != wantBuildings {
		t.Errorf("expected one lod2_building row per CityGML Building (%d), got %d", wantBuildings, buildingCount)
	}
	if partKeyed > 0 {
		t.Errorf("%d lod2_building rows are not keyed by their Building's feature id and object_id", partKeyed)
	}

	// surface_count_{roof,wall,floor} count every served envelope row; party-wall
	// pieces are served too but are not envelope.
	var surfaceCountMismatch int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM city2tabula.lod2_building b
		 WHERE b.surface_count_roof + b.surface_count_wall + b.surface_count_floor
		     <> (SELECT COUNT(*) FROM city2tabula.lod2_surface s
		         WHERE s.building_object_id = b.object_id AND NOT s.is_party_wall)`,
	).Scan(&surfaceCountMismatch); err != nil {
		t.Fatalf("failed to query surface-count consistency: %v", err)
	}
	if surfaceCountMismatch > 0 {
		t.Errorf("surface counts do not sum to the served envelope rows for %d buildings", surfaceCountMismatch)
	}

	// Served envelope walls add up to area_total_wall; each value is rounded to 2
	// decimals on its own, hence the per-face tolerance.
	var wallAreaMismatch int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM city2tabula.lod2_building b
		 JOIN (SELECT building_object_id, SUM(surface_area) AS area, COUNT(*) AS n
		       FROM city2tabula.lod2_surface WHERE surface_type = 'WallSurface' AND NOT is_party_wall
		       GROUP BY building_object_id) s ON s.building_object_id = b.object_id
		 WHERE ABS(s.area - b.area_total_wall) > 0.01 * s.n + 0.01`,
	).Scan(&wallAreaMismatch); err != nil {
		t.Fatalf("failed to query wall-area consistency: %v", err)
	}
	if wallAreaMismatch > 0 {
		t.Errorf("served wall area differs from area_total_wall for %d buildings", wallAreaMismatch)
	}

	// Verify the surface builder filled lod2_surface.
	var surfaceLinkCount int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_surface",
	).Scan(&surfaceLinkCount); err != nil {
		t.Fatalf("failed to query lod2_surface: %v", err)
	}
	if surfaceLinkCount == 0 {
		t.Error("surface builder failed: lod2_surface is empty, expected surface rows")
	}

	// Every served wall, roof and ground piece carries an in-plane
	// length and width, with the short side stored as width.
	var badDims int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM city2tabula.lod2_surface
		WHERE surface_type IN ('WallSurface', 'RoofSurface', 'GroundSurface')
		  AND (length IS NULL OR width IS NULL OR width > length)`,
	).Scan(&badDims); err != nil {
		t.Fatalf("failed to query surface dimensions: %v", err)
	}
	if badDims > 0 {
		t.Errorf("%d surfaces have missing length/width or width > length", badDims)
	}

	// Regression (#121): a multi-face surface feature (a 3DBAG WallSurface covers all of a
	// building's walls) must not collapse to one row. lod2_surface holds one row per
	// exterior and per party-wall piece of every raw face of a served building.
	var rawEligible, resolved int
	if err := testPool.QueryRow(ctx, `
		SELECT
			(SELECT COALESCE(SUM(COALESCE(ST_NumGeometries(COALESCE(geom_envelope, geom_exposed)), 1)
			                     + COALESCE(ST_NumGeometries(geom_party), 0)), 0)
			 FROM city2tabula.lod2_surface_raw sr
			 WHERE building_object_id IS NOT NULL AND surface_object_id IS NOT NULL
			   AND EXISTS (SELECT 1 FROM city2tabula.lod2_building b
			               WHERE b.building_feature_id = sr.building_feature_id)),
			(SELECT COUNT(*) FROM city2tabula.lod2_surface)
	`).Scan(&rawEligible, &resolved); err != nil {
		t.Fatalf("failed to compare raw vs resolved surface counts: %v", err)
	}
	if resolved != rawEligible {
		t.Errorf("surface builder dropped faces: lod2_surface has %d rows, expected %d (all pieces of eligible raw faces)", resolved, rawEligible)
	}

	t.Logf("pipeline complete: %d buildings processed, %d labeled with TABULA codes, %d surface links, %d surfaces with objectids",
		buildingCount, labeledCount, surfaceLinkCount, missingSurfaceObjectIDs)
}

func TestPipeline_Germany_LOD2(t *testing.T) {
	runPipelineTest(t, pipelineTestCase{
		country: "germany",
		srid:    "25832",
		seeds: []string{
			"testdata/germany/seed_lod2.sql",
			"testdata/germany/seed_tabula_variant.sql",
		},
	})
}

func TestPipeline_Austria_LOD2(t *testing.T) {
	runPipelineTest(t, pipelineTestCase{
		country: "austria",
		srid:    "31256", // MGI / Austria GK East — verify from your AT CityDB import settings
		seeds: []string{
			"testdata/austria/seed_lod2.sql",
			"testdata/austria/seed_tabula_variant.sql",
		},
	})
}

func TestPipeline_Netherlands_LOD2(t *testing.T) {
	runPipelineTest(t, pipelineTestCase{
		country: "netherlands",
		srid:    "28992", // Amersfoort / RD New — verify from your NL CityDB import settings
		seeds: []string{
			"testdata/netherlands/seed_lod2.sql",
			"testdata/netherlands/seed_tabula_variant.sql",
		},
	})
}
