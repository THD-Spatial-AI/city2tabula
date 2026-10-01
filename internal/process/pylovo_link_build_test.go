//go:build integration

package process_test

import (
	"context"
	"testing"

	"bytes"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
	"strings"
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
// owned by the external enerplanet-pylovo database, not this repo) under the
// "public" schema, matching the columns 01_build_pylovo_link.sql reads: osm_id,
// country_code and geom in EPSG:3035 (PyLovo's native CRS, per that script's own
// comments).
func seedPylovoTestTables(t *testing.T, ctx context.Context) {
	t.Helper()
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS public.res CASCADE`,
		`DROP TABLE IF EXISTS public.oth CASCADE`,
		`CREATE TABLE public.res (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
		`CREATE TABLE public.oth (osm_id TEXT, country_code VARCHAR(2), geom GEOMETRY(MultiPolygon, 3035))`,
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
// The row carries the building's own country_code, as PyLovo's would.
func insertPylovoRowFromBuilding(t *testing.T, ctx context.Context, table, osmID, objectID string) {
	t.Helper()
	insertPylovoRowWithCountry(t, ctx, table, osmID, objectID, "")
}

// insertPylovoRowWithCountry is insertPylovoRowFromBuilding with the PyLovo row's
// country_code set to countryCode instead of the building's, when it is not empty.
func insertPylovoRowWithCountry(t *testing.T, ctx context.Context, table, osmID, objectID, countryCode string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `
		INSERT INTO public.`+table+` (osm_id, country_code, geom)
		SELECT $1, COALESCE(NULLIF($3, ''), country_code),
		       ST_Multi(ST_Force2D(ST_Transform(building_footprint_geom, 3035)))
		FROM city2tabula.lod2_building
		WHERE object_id = $2`,
		osmID, objectID, countryCode,
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
	// Same footprint as the unmatched building but another country's row: PyLovo
	// holds every country in one table, so the link must filter on country_code.
	insertPylovoRowWithCountry(t, ctx, "res", "RES-OTHER-COUNTRY", buildingUnmatched, "NL")

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

// captureWarn redirects utils.Warn into a buffer for the rest of the test.
func captureWarn(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := utils.Warn.Writer()
	utils.Warn.SetOutput(&buf)
	t.Cleanup(func() { utils.Warn.SetOutput(prev) })
	return &buf
}

// TestRunPyLovoLinkBuild_ReportsAlreadyLinked pins the message of a run that
// finds every building already linked: it must say so, not claim the buildings
// have no footprints.
func TestRunPyLovoLinkBuild_ReportsAlreadyLinked(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000
	seedPylovoTestTables(t, ctx)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("first RunPyLovoLinkBuild: %v", err)
	}
	warn := captureWarn(t)
	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("second RunPyLovoLinkBuild: %v", err)
	}
	if got := warn.String(); !strings.Contains(got, "already have a building_link row") {
		t.Errorf("second run should report the buildings as already linked, logged:\n%s", got)
	}
}

// TestRunPyLovoRelink_UpdatesExistingLinks pins the re-link path: after the PyLovo
// row a building matched is replaced, a plain link run leaves the old link alone
// and RunPyLovoRelink replaces it with the new match.
func TestRunPyLovoRelink_UpdatesExistingLinks(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000
	seedPylovoTestTables(t, ctx)
	building, _, _ := pylovoLinkFixtureBuildings(t, ctx)

	insertPylovoRowFromBuilding(t, ctx, "res", "RES-OLD", building)
	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("first RunPyLovoLinkBuild: %v", err)
	}
	if got := readBuildingLink(t, ctx, building); got.osmID == nil || *got.osmID != "RES-OLD" {
		t.Fatalf("expected %s linked to RES-OLD after the first run, got %s", building, osmIDString(got.osmID))
	}

	if _, err := testPool.Exec(ctx, `DELETE FROM public.res WHERE osm_id = 'RES-OLD'`); err != nil {
		t.Fatalf("remove RES-OLD: %v", err)
	}
	insertPylovoRowFromBuilding(t, ctx, "res", "RES-NEW", building)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("second RunPyLovoLinkBuild: %v", err)
	}
	if got := readBuildingLink(t, ctx, building); got.osmID == nil || *got.osmID != "RES-OLD" {
		t.Errorf("a plain link run must leave the existing link alone, got %s", osmIDString(got.osmID))
	}

	if err := process.RunPyLovoRelink(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoRelink: %v", err)
	}
	if got := readBuildingLink(t, ctx, building); got.osmID == nil || *got.osmID != "RES-NEW" {
		t.Errorf("expected RunPyLovoRelink to re-link %s to RES-NEW, got %s", building, osmIDString(got.osmID))
	}
}

// osmIDString renders a nullable osm_id for test messages.
func osmIDString(id *string) string {
	if id == nil {
		return "<nil>"
	}
	return *id
}
