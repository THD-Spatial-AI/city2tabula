//go:build integration

package process_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestRunPyLovoLinkBuild_ViaFDW exercises the postgres_fdw path: pylovo.res/oth
// live in a separate database, and RunPyLovoLinkBuild wires the foreign server,
// user mapping and foreign schema before the link runs. The FDW server points
// the container's postgres process back at itself (localhost:5432), so no second
// container is needed.
func TestRunPyLovoLinkBuild_ViaFDW(t *testing.T) {
	ctx := context.Background()
	const srcDB = "pylovo_fdw_src"

	// Sibling database holding the real pylovo tables.
	dropSiblingDB(t, ctx, srcDB)
	if _, err := testPool.Exec(ctx, `CREATE DATABASE `+srcDB); err != nil {
		t.Fatalf("create sibling db: %v", err)
	}
	t.Cleanup(func() { dropSiblingDB(t, context.Background(), srcDB) })

	srcConnStr := strings.Replace(testConnStr, "/city2tabula_test?", "/"+srcDB+"?", 1)
	srcPool, err := pgxpool.New(ctx, srcConnStr)
	if err != nil {
		t.Fatalf("connect sibling db: %v", err)
	}
	defer srcPool.Close()
	for _, stmt := range []string{
		`CREATE EXTENSION IF NOT EXISTS postgis`,
		`CREATE TABLE public.res (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
		`CREATE TABLE public.oth (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
	} {
		if _, err := srcPool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare sibling pylovo tables (%s): %v", stmt, err)
		}
	}

	cfg, _ := setupCorrectionAuditFixture(t)
	cfg.City2Tabula.LinkGridSize = 1000
	cfg.DB.Schemas.Pylvo = config.PylvoFDWSchemaName
	cfg.PylovoFDW = &config.PylovoFDW{
		Host: "localhost", Port: "5432", DBName: srcDB, User: "test", Password: "test",
	}

	matched, unmatched, _ := pylovoLinkFixtureBuildings(t, ctx)
	seedForeignPylovoRow(t, ctx, srcPool, "res", "FDW-MATCH", matched)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild via FDW: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DROP SERVER IF EXISTS pylovo_srv CASCADE`)
	})

	got := readBuildingLink(t, ctx, matched)
	if got.matchType != 1 || got.osmID == nil || *got.osmID != "FDW-MATCH" {
		t.Errorf("expected %s to match FDW-MATCH via foreign pylovo.res, got match_type=%d osm_id=%v",
			matched, got.matchType, got.osmID)
	}
	if none := readBuildingLink(t, ctx, unmatched); none.matchType != 2 {
		t.Errorf("expected %s unmatched (match_type=2), got %d", unmatched, none.matchType)
	}

	// Both pre-filters must reach PyLovo, or every batch pulls every country's
	// res/oth rows across the connection and filters them locally.
	for _, table := range []string{"res", "oth"} {
		remote := remoteSQLFor(t, ctx, explainLinkInsert(t, ctx, cfg, matched), table)
		for _, want := range []string{"st_intersects(geom, $", "country_code = '" + cfg.CountryCode + "'"} {
			if !strings.Contains(remote, want) {
				t.Errorf("remote query for %s does not contain %q:\n%s", table, want, remote)
			}
		}
	}
}

// explainLinkInsert returns EXPLAIN VERBOSE output for the INSERT in
// 01_build_pylovo_link.sql, with its placeholders filled for one LOD2 building.
// EXPLAIN without ANALYZE plans the statement without running it.
func explainLinkInsert(t *testing.T, ctx context.Context, cfg *config.Config, objectID string) string {
	t.Helper()
	script, err := os.ReadFile("sql/scripts/link/pylovo/01_build_pylovo_link.sql")
	if err != nil {
		t.Fatalf("read link script: %v", err)
	}
	var featureID int64
	if err := testPool.QueryRow(ctx,
		`SELECT building_feature_id FROM city2tabula.lod2_building WHERE object_id = $1`, objectID,
	).Scan(&featureID); err != nil {
		t.Fatalf("read building_feature_id for %s: %v", objectID, err)
	}
	sql := strings.NewReplacer(
		"{city2tabula_schema}", cfg.DB.Schemas.City2Tabula,
		"{lod_schema}", cfg.DB.Schemas.Lod2,
		"{building_ids}", fmt.Sprintf("(%d)", featureID),
		"{pylovo_schema}", cfg.DB.Schemas.Pylvo,
		"{srid}", cfg.CityDB.SRID,
		"{country_code}", cfg.CountryCode,
	).Replace(string(script))

	start := strings.Index(sql, "WITH buildings AS")
	if start < 0 {
		t.Fatal("link script has no WITH buildings AS statement")
	}
	rows, err := testPool.Query(ctx, "EXPLAIN (VERBOSE) "+strings.TrimSuffix(strings.TrimSpace(sql[start:]), ";"))
	if err != nil {
		t.Fatalf("EXPLAIN link insert: %v", err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan EXPLAIN line: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("EXPLAIN link insert: %v", err)
	}
	return plan.String()
}

// remoteSQLFor returns the Remote SQL line of the foreign scan on public.<table>.
func remoteSQLFor(t *testing.T, ctx context.Context, plan, table string) string {
	t.Helper()
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "Remote SQL:") && strings.Contains(line, "FROM public."+table) {
			return line
		}
	}
	t.Fatalf("no foreign scan on public.%s in plan:\n%s", table, plan)
	return ""
}

// seedForeignPylovoRow copies a fixture building's own footprint (transformed to
// EPSG:3035) into pylovo.res/oth in the sibling database, giving a near-exact IoU
// match by construction.
func seedForeignPylovoRow(t *testing.T, ctx context.Context, srcPool *pgxpool.Pool, table, osmID, objectID string) {
	t.Helper()
	var ewkt, countryCode string
	if err := testPool.QueryRow(ctx, `
		SELECT ST_AsEWKT(ST_Multi(ST_Force2D(ST_Transform(building_footprint_geom, 3035)))), country_code
		FROM city2tabula.lod2_building WHERE object_id = $1`, objectID,
	).Scan(&ewkt, &countryCode); err != nil {
		t.Fatalf("read fixture footprint for %s: %v", objectID, err)
	}
	if _, err := srcPool.Exec(ctx,
		`INSERT INTO public.`+table+` (osm_id, country_code, geom) VALUES ($1, $2, ST_GeomFromEWKT($3))`,
		osmID, countryCode, ewkt,
	); err != nil {
		t.Fatalf("seed foreign pylovo.%s row: %v", table, err)
	}
}

// dropSiblingDB force-drops a database, terminating any lingering backends first.
func dropSiblingDB(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	testPool.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, name)
	if _, err := testPool.Exec(ctx, `DROP DATABASE IF EXISTS `+name); err != nil {
		t.Fatalf("drop sibling db %s: %v", name, err)
	}
}
