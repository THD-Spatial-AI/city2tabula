//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/api/handler"
	"github.com/thd-spatial-ai/city2tabula/internal/api/router"
	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// This file tests the HTTP layer (handler/router/server wiring, JSON shapes,
// query parsing) against directly-seeded data. It deliberately does not drive
// a real pipeline run through the HTTP /runs endpoint — RunForRegion and
// CountBuildingLink/BuildingsByOSMIDs themselves are covered end to end,
// against a real citydb-tool binary, by
// internal/onrequest's TestRunForRegion_RealCitydbTool_ImportsAndLinksBuildings.
// Server.StartRun's goroutine/status-transition wiring around that call is not
// yet covered by an automated test — flagged as a known gap, not silently
// skipped.

// baseServerConfig builds the process-wide config a server.Server needs, by
// hand (bypasses config.LoadEnv/.env — see onrequest's e2eConfig for why).
func baseServerConfig(host, port, dbNamePrefix string) config.Config {
	return config.Config{
		DB: &config.DBConfig{
			Host:     host,
			Port:     port,
			Name:     dbNamePrefix,
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
				Pylvo:       config.PylvoSchemaName,
			},
		},
		Data:        &config.DataPaths{Base: config.DataDir},
		CityDB:      &config.CityDB{LODLevels: []int{2}},
		City2Tabula: &config.City2TabulaConfig{RoomHeight: "2.5", LinkGridSize: 1000},
		Batch:       &config.BatchConfig{Size: 100, Threads: 2},
		RetryConfig: config.DefaultRetryConfig(),
	}
}

// seedViaPsql executes a pg_dump-style SQL file (COPY FROM stdin, which pgx
// can't run directly) via psql — same approach internal/process's seedDB
// helper uses for the identical fixture files.
func seedViaPsql(t *testing.T, host, port, dbName, path string) {
	t.Helper()
	connStr := "postgres://" + testutil.TestUser + ":" + testutil.TestPassword + "@" + host + ":" + port + "/" + dbName
	cmd := exec.Command("psql", connStr, "-v", "ON_ERROR_STOP=1", "-f", path)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+testutil.TestPassword)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to seed %s: %v\npsql output:\n%s", path, err, string(out))
	}
}

func TestServer_Coverage_And_Buildings(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	projectRoot := filepath.Join(wd, "..", "..") // internal/api -> project root
	t.Chdir(projectRoot)

	seedPath := "testdata/germany/seed_lod2.sql"
	if _, err := os.Stat(seedPath); err != nil {
		t.Skipf("%s not found, skipping: %v", seedPath, err)
	}

	host, port := testutil.StartPostGISAddr(t)
	base := baseServerConfig(host, port, "api_test")
	cfg, err := config.RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	cfg.Data = &config.DataPaths{Tabula: filepath.Join(projectRoot, "data", "tabula") + string(filepath.Separator)}

	pool, err := db.ConnectPool(&cfg)
	if err != nil {
		t.Fatalf("ConnectPool: %v", err)
	}
	t.Cleanup(func() { db.ClosePool(pool) })

	if err := db.RunCity2TabulaDBSetup(&cfg, pool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	seedViaPsql(t, host, port, cfg.DB.Name, seedPath)

	if err := process.RunFeatureExtraction(&cfg, pool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var objectID string
	if err := pool.QueryRow(context.Background(),
		`SELECT object_id FROM city2tabula.lod2_building WHERE object_id IS NOT NULL LIMIT 1`,
	).Scan(&objectID); err != nil {
		t.Fatalf("failed to pick a fixture building: %v", err)
	}

	const osmID = "test-osm-id-1"
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO city2tabula.building_link (object_id, osm_id, match_type, pylovo_table, match_confidence, country_code, srid, geom)
		SELECT object_id, $1, 1, 'res', 0.9, country_code, $3::int, ST_Multi(ST_Force2D(building_footprint_geom))
		FROM city2tabula.lod2_building WHERE object_id = $2`,
		osmID, objectID, cfg.CityDB.SRID,
	); err != nil {
		t.Fatalf("failed to seed building_link: %v", err)
	}

	// WGS84 bbox tightly around the seeded building's own footprint - a
	// whole-world envelope doesn't survive ST_Transform into a UTM zone (the
	// projection isn't defined that far from its zone), which isn't a
	// realistic query shape anyway (real callers pass a local area of interest).
	var xmin, ymin, xmax, ymax float64
	if err := pool.QueryRow(context.Background(), `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (
			SELECT ST_Transform(ST_Envelope(geom), 4326) AS e
			FROM city2tabula.building_link WHERE object_id = $1
		) t`,
		objectID,
	).Scan(&xmin, &ymin, &xmax, &ymax); err != nil {
		t.Fatalf("failed to compute WGS84 bbox for seeded building: %v", err)
	}
	const pad = 0.001 // ~100m, comfortably covers the one building without pulling in the whole fixture
	xmin, ymin, xmax, ymax = xmin-pad, ymin-pad, xmax+pad, ymax+pad

	srv := server.New(base)
	h := handler.New(srv)
	ts := httptest.NewServer(router.New(h))
	defer ts.Close()

	// Coverage: a bbox tight around the seeded building should find it.
	coverageURL := fmt.Sprintf("%s/api/v1/coverage?country=germany&xmin=%f&ymin=%f&xmax=%f&ymax=%f",
		ts.URL, xmin, ymin, xmax, ymax)
	resp, err := http.Get(coverageURL)
	if err != nil {
		t.Fatalf("GET /coverage: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /coverage: status = %d, want 200", resp.StatusCode)
	}
	var coverage struct{ Count int `json:"count"` }
	if err := json.NewDecoder(resp.Body).Decode(&coverage); err != nil {
		t.Fatalf("decode /coverage response: %v", err)
	}
	if coverage.Count < 1 {
		t.Errorf("coverage.Count = %d, want >= 1", coverage.Count)
	}

	// Buildings: querying by the seeded osm_id should return exactly one row.
	resp2, err := http.Get(ts.URL + "/api/v1/buildings?country=germany&osm_ids=" + osmID)
	if err != nil {
		t.Fatalf("GET /buildings: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /buildings: status = %d, want 200", resp2.StatusCode)
	}
	var buildings []struct {
		ObjectID string `json:"object_id"`
		OSMID    string `json:"osm_id"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&buildings); err != nil {
		t.Fatalf("decode /buildings response: %v", err)
	}
	if len(buildings) != 1 {
		t.Fatalf("expected exactly 1 building, got %d", len(buildings))
	}
	if buildings[0].ObjectID != objectID || buildings[0].OSMID != osmID {
		t.Errorf("buildings[0] = %+v, want object_id=%q osm_id=%q", buildings[0], objectID, osmID)
	}

	// Buildings by bbox: same seeded building, found without any osm_id/PyLovo
	// link — a fresh region with no building_link rows yet must still be able
	// to serve envelope data by bbox alone.
	bboxURL := fmt.Sprintf("%s/api/v1/buildings?country=germany&xmin=%f&ymin=%f&xmax=%f&ymax=%f",
		ts.URL, xmin, ymin, xmax, ymax)
	resp4, err := http.Get(bboxURL)
	if err != nil {
		t.Fatalf("GET /buildings?bbox: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("GET /buildings?bbox: status = %d, want 200", resp4.StatusCode)
	}
	var buildingsByBBox []struct {
		ObjectID string `json:"object_id"`
		OSMID    string `json:"osm_id"`
	}
	if err := json.NewDecoder(resp4.Body).Decode(&buildingsByBBox); err != nil {
		t.Fatalf("decode /buildings?bbox response: %v", err)
	}
	if len(buildingsByBBox) != 1 {
		t.Fatalf("expected exactly 1 building, got %d", len(buildingsByBBox))
	}
	if buildingsByBBox[0].ObjectID != objectID {
		t.Errorf("buildingsByBBox[0].ObjectID = %q, want %q", buildingsByBBox[0].ObjectID, objectID)
	}
	if buildingsByBBox[0].OSMID != "" {
		t.Errorf("buildingsByBBox[0].OSMID = %q, want empty — bbox mode doesn't join building_link", buildingsByBBox[0].OSMID)
	}

	// RunStatus for an unknown id: 404.
	resp3, err := http.Get(ts.URL + "/api/v1/runs/does-not-exist")
	if err != nil {
		t.Fatalf("GET /runs/does-not-exist: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("GET /runs/does-not-exist: status = %d, want 404", resp3.StatusCode)
	}
}
