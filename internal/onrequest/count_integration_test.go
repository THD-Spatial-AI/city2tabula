//go:build integration

package onrequest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// countTestSRID is Germany's SRID; the fixture geometries below are written in it.
const countTestSRID = "25832"

// countTestConfig carries only the four fields CountBuildingLink reads, so this
// test needs neither citydb-tool nor the TABULA CSVs.
func countTestConfig() *config.Config {
	return &config.Config{
		Country:     "germany",
		CountryCode: "DE",
		DB:          &config.DBConfig{Schemas: &config.Schemas{City2Tabula: config.City2TabulaSchema}},
		CityDB:      &config.CityDB{SRID: countTestSRID},
	}
}

// createLinkTable applies the real schema script rather than hand-rolled DDL, so
// a column or constraint change there surfaces here.
func createLinkTable(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	path := filepath.Join(wd, "..", "..", "sql", "schema", "main", "02_create_link_tables.sql")
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+config.City2TabulaSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	sql := strings.ReplaceAll(string(script), "{city2tabula_schema}", config.City2TabulaSchema)
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("apply link table schema: %v", err)
	}
}

// insertLink adds one building_link row whose footprint is a 1m square at
// (x, y) in EPSG:25832.
func insertLink(t *testing.T, pool *pgxpool.Pool, objectID string, matchType int, osmID *string, x, y int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO city2tabula.building_link
			(object_id, osm_id, match_type, country_code, geom, srid)
		VALUES ($1, $2, $3, 'DE',
			ST_Multi(ST_MakeEnvelope($4::float8, $5::float8, $4::float8 + 1, $5::float8 + 1, `+countTestSRID+`)),
			`+countTestSRID+`)`,
		objectID, osmID, matchType, x, y,
	)
	if err != nil {
		t.Fatalf("insert %s: %v", objectID, err)
	}
}

// TestCountBuildingLink_CountsOnlyMatchedBuildings pins the meaning of the
// coverage count: it reports buildings that were linked to a PyLovo building,
// not buildings the linker attempted. The linker writes one row per 3D building
// regardless of outcome (sql/scripts/link/pylovo/01_build_pylovo_link.sql), so
// an unfiltered count reports an area where nothing matched as fully covered.
func TestCountBuildingLink_CountsOnlyMatchedBuildings(t *testing.T) {
	pool := testutil.StartPostGIS(t)
	createLinkTable(t, pool)

	cfg := countTestConfig()
	osmID := "osm-1"

	// Bremen, inside the bbox asserted below.
	insertLink(t, pool, "matched-1", 1, &osmID, 487000, 5883000)
	insertLink(t, pool, "unmatched-1", 2, nil, 487010, 5883000)
	insertLink(t, pool, "unmatched-2", 2, nil, 487020, 5883000)
	// OSM-only: no 3D building behind it, so not modellable either.
	insertLink(t, pool, "osm-only-1", 3, &osmID, 487030, 5883000)

	bbox := onrequest.Bbox{Xmin: 8.78, Ymin: 53.08, Xmax: 8.83, Ymax: 53.11}
	count, err := onrequest.CountBuildingLink(context.Background(), pool, cfg, bbox)
	if err != nil {
		t.Fatalf("CountBuildingLink: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 matched building, got %d (unmatched rows are being counted as coverage)", count)
	}
}

// TestCountBuildingLink_AllUnmatchedReportsZero is the case that inverts the
// feature: buildings exist and the linker ran, but nothing matched, so there is
// nothing to model and coverage must say so.
func TestCountBuildingLink_AllUnmatchedReportsZero(t *testing.T) {
	pool := testutil.StartPostGIS(t)
	createLinkTable(t, pool)

	for _, id := range []string{"unmatched-1", "unmatched-2", "unmatched-3"} {
		insertLink(t, pool, id, 2, nil, 487000, 5883000)
	}

	bbox := onrequest.Bbox{Xmin: 8.78, Ymin: 53.08, Xmax: 8.83, Ymax: 53.11}
	count, err := onrequest.CountBuildingLink(context.Background(), pool, countTestConfig(), bbox)
	if err != nil {
		t.Fatalf("CountBuildingLink: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for an area where nothing matched, got %d", count)
	}
}
