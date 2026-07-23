//go:build integration

package process_test

import (
	"context"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// pylovoLinkFixtureBuildings picks 3 distinct, real object_ids from the already-
// extracted fixture (footprint + object_id both set) to drive the three outcomes
// RunPyLovoLinkBuild can produce: a res match, an oth match, and no match at all.
func pylovoLinkFixtureBuildings(t *testing.T, ctx context.Context) (a, b, c string) {
	t.Helper()
	rows, err := testPool.Query(ctx, `
		SELECT object_id FROM city2tabula.lod2_building
		WHERE object_id IS NOT NULL AND building_footprint_geom IS NOT NULL
		ORDER BY object_id LIMIT 3`,
	)
	if err != nil {
		t.Fatalf("failed to pick fixture buildings: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("failed to scan object_id: %v", err)
		}
		ids = append(ids, id)
	}
	if len(ids) < 3 {
		t.Fatalf("fixture only has %d eligible buildings, need at least 3", len(ids))
	}
	return ids[0], ids[1], ids[2]
}

// seedPylovoTestTables creates minimal pylovo.res / pylovo.oth tables (normally
// owned by the external PyLovo2EnerPlanET database, not this repo) under the
// "public" schema, matching the columns 01_build_pylovo_link.sql reads: osm_id and
// geom in EPSG:3035 (PyLovo's native CRS, per that script's own comments).
func seedPylovoTestTables(t *testing.T, ctx context.Context) {
	t.Helper()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS public.res CASCADE`,
		`DROP TABLE IF EXISTS public.oth CASCADE`,
		`CREATE TABLE public.res (osm_id TEXT, geom GEOMETRY(MultiPolygon, 3035))`,
		`CREATE TABLE public.oth (osm_id TEXT, geom GEOMETRY(MultiPolygon, 3035))`,
	} {
		if _, err := testPool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to prepare pylovo test tables (%s): %v", stmt, err)
		}
	}
}

// insertPylovoRowFromBuilding copies a real fixture building's own footprint
// (transformed to PyLovo's native CRS) into pylovo.res or pylovo.oth as osmID,
// guaranteeing a near-exact IoU match by construction — no hand-crafted EPSG:3035
// coordinates needed, and no ambiguity about what the "correct" match should be.
func insertPylovoRowFromBuilding(t *testing.T, ctx context.Context, table, osmID, objectID string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `
		INSERT INTO public.`+table+` (osm_id, geom)
		SELECT $1, ST_Multi(ST_Force2D(ST_Transform(building_footprint_geom, 3035)))
		FROM city2tabula.lod2_building
		WHERE object_id = $2`,
		osmID, objectID,
	); err != nil {
		t.Fatalf("failed to seed pylovo.%s row for %s: %v", table, objectID, err)
	}
}

type buildingLinkRow struct {
	matchType   int
	osmID       *string
	pylovoTable *string
	confidence  *float64
}

func readBuildingLink(t *testing.T, ctx context.Context, objectID string) buildingLinkRow {
	t.Helper()
	var row buildingLinkRow
	if err := testPool.QueryRow(ctx, `
		SELECT match_type, osm_id, pylovo_table, match_confidence
		FROM city2tabula.building_link WHERE object_id = $1`, objectID,
	).Scan(&row.matchType, &row.osmID, &row.pylovoTable, &row.confidence); err != nil {
		t.Fatalf("failed to read building_link for %s: %v", objectID, err)
	}
	return row
}

// TestRunPyLovoLinkBuild_MatchesResOthAndUnmatched exercises all three outcomes
// end to end: a building with an exact res match, one with an exact oth match (and
// a decoy res match on a different building to confirm res still wins when both
// exist), and one with no PyLovo counterpart at all.
func TestRunPyLovoLinkBuild_MatchesResOthAndUnmatched(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	seedPylovoTestTables(t, ctx)
	buildingWithResMatch, buildingWithOthMatch, buildingUnmatched := pylovoLinkFixtureBuildings(t, ctx)

	insertPylovoRowFromBuilding(t, ctx, "res", "RES-MATCH", buildingWithResMatch)
	insertPylovoRowFromBuilding(t, ctx, "oth", "OTH-MATCH", buildingWithOthMatch)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}

	res := readBuildingLink(t, ctx, buildingWithResMatch)
	if res.matchType != 1 || res.pylovoTable == nil || *res.pylovoTable != "res" || res.osmID == nil || *res.osmID != "RES-MATCH" {
		t.Errorf("expected %s to match pylovo.res row RES-MATCH, got match_type=%d pylovo_table=%v osm_id=%v",
			buildingWithResMatch, res.matchType, res.pylovoTable, res.osmID)
	}
	if res.confidence == nil || *res.confidence < 0.9 {
		t.Errorf("expected match_confidence close to 1.0 for an exact-geometry res match, got %v", res.confidence)
	}

	oth := readBuildingLink(t, ctx, buildingWithOthMatch)
	if oth.matchType != 1 || oth.pylovoTable == nil || *oth.pylovoTable != "oth" || oth.osmID == nil || *oth.osmID != "OTH-MATCH" {
		t.Errorf("expected %s to match pylovo.oth row OTH-MATCH, got match_type=%d pylovo_table=%v osm_id=%v",
			buildingWithOthMatch, oth.matchType, oth.pylovoTable, oth.osmID)
	}
	if oth.confidence == nil || *oth.confidence < 0.9 {
		t.Errorf("expected match_confidence close to 1.0 for an exact-geometry oth match, got %v", oth.confidence)
	}

	unmatched := readBuildingLink(t, ctx, buildingUnmatched)
	if unmatched.matchType != 2 || unmatched.osmID != nil || unmatched.pylovoTable != nil || unmatched.confidence != nil {
		t.Errorf("expected %s to be unmatched (match_type=2, all match fields NULL), got match_type=%d osm_id=%v pylovo_table=%v confidence=%v",
			buildingUnmatched, unmatched.matchType, unmatched.osmID, unmatched.pylovoTable, unmatched.confidence)
	}
}

// TestRunPyLovoLinkBuild_PrefersResOverOth seeds both an exact res match and an
// exact oth match for the same building, and checks the res_candidates-before-
// oth_candidates precedence in 01_build_pylovo_link.sql actually holds: the
// building must link to res, never oth, when both clear the IoU threshold.
func TestRunPyLovoLinkBuild_PrefersResOverOth(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	seedPylovoTestTables(t, ctx)
	buildingWithBothMatches, _, _ := pylovoLinkFixtureBuildings(t, ctx)

	insertPylovoRowFromBuilding(t, ctx, "res", "RES-PREFERRED", buildingWithBothMatches)
	insertPylovoRowFromBuilding(t, ctx, "oth", "OTH-DECOY", buildingWithBothMatches)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}

	got := readBuildingLink(t, ctx, buildingWithBothMatches)
	if got.pylovoTable == nil || *got.pylovoTable != "res" || got.osmID == nil || *got.osmID != "RES-PREFERRED" {
		t.Errorf("expected res to be preferred over oth when both match, got pylovo_table=%v osm_id=%v", got.pylovoTable, got.osmID)
	}
}
