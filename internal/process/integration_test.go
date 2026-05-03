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

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get container port: %v", err)
	}

	testConnStr = fmt.Sprintf("postgres://test:test@%s:%s/city2tabula_test?sslmode=disable", host, port.Port())

	testPool, err = pgxpool.New(ctx, testConnStr)
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
	return &config.Config{
		Country: tc.country,
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
			RoomHeight: "2.5",
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
			TRUNCATE city2tabula.lod2_building_feature         CASCADE;
			TRUNCATE city2tabula.lod2_child_feature            CASCADE;
			TRUNCATE city2tabula.lod2_child_feature_surface    CASCADE;
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

// seedDB executes one or more SQL files via psql.
// pg_dump files use COPY FROM stdin which pool.Exec() cannot handle —
// psql processes the COPY protocol correctly.
func seedDB(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		// ON_ERROR_STOP=1 makes psql exit non-zero on any SQL error so we catch failures.
		cmd := exec.Command("psql", testConnStr, "-v", "ON_ERROR_STOP=1", "-f", path)
		cmd.Env = append(os.Environ(), "PGPASSWORD=test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to seed %s: %v\npsql output:\n%s", path, err, string(out))
		}
	}
}

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
		"SELECT COUNT(*) FROM city2tabula.lod2_building_feature",
	).Scan(&buildingCount); err != nil {
		t.Fatalf("failed to query building count: %v", err)
	}
	if buildingCount == 0 {
		t.Error("expected buildings in city2tabula.lod2_building_feature, got 0")
	}

	var labeledCount int
	if err := testPool.QueryRow(ctx,
		"SELECT COUNT(*) FROM city2tabula.lod2_building_feature WHERE tabula_variant_code IS NOT NULL",
	).Scan(&labeledCount); err != nil {
		t.Fatalf("failed to query labeled count: %v", err)
	}
	if labeledCount == 0 {
		t.Error("expected at least 1 building to have a TABULA variant code assigned")
	}

	t.Logf("pipeline complete: %d buildings processed, %d labeled with TABULA codes",
		buildingCount, labeledCount)
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
