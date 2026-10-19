//go:build integration

package process_test

import (
	"context"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// bboxAround returns the EPSG:4326 envelope of a building's footprint, grown by a
// metre so the bbox query finds it.
func bboxAround(t *testing.T, ctx context.Context, objectID string) onrequest.Bbox {
	t.Helper()
	var b onrequest.Bbox
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_Expand(ST_Envelope(building_footprint_geom), 1), 4326) AS e
		      FROM city2tabula.lod2_building WHERE object_id = $1) x`, objectID,
	).Scan(&b.Xmin, &b.Ymin, &b.Xmax, &b.Ymax); err != nil {
		t.Fatalf("bbox of %s: %v", objectID, err)
	}
	return b
}

// show prints a nullable string for a test message.
func show(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

// functionCodeOf finds objectID in buildings and returns its FunctionCode.
func functionCodeOf(t *testing.T, buildings []onrequest.Building, objectID string) *string {
	t.Helper()
	for _, b := range buildings {
		if b.ObjectID == objectID {
			return b.FunctionCode
		}
	}
	t.Fatalf("%s not in the response (%d buildings)", objectID, len(buildings))
	return nil
}

// TestServe_FunctionCodeFromSourceData pins that both building queries return the
// source Building's CityGML function code (the seed is real Bavarian data, which carries
// one per Building), null for a building without one, and null rather than an error
// when the source property table is missing.
func TestServe_FunctionCodeFromSourceData(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	coded, uncoded, _ := pylovoLinkFixtureBuildings(t, ctx)
	var want string
	if err := testPool.QueryRow(ctx, `
		SELECT p.val_string FROM lod2.property p JOIN lod2.feature f ON f.id = p.feature_id
		WHERE f.objectid = $1 AND f.objectclass_id = 901 AND p.name = 'function'`, coded,
	).Scan(&want); err != nil || want == "" {
		t.Fatalf("seed building %s has no function code to serve (%q, %v)", coded, want, err)
	}
	mustExec(t, ctx, `
		DELETE FROM lod2.property WHERE name = 'function'
		  AND feature_id IN (SELECT id FROM lod2.feature WHERE objectid = '`+uncoded+`')`)

	seedPylovoTestTables(t, ctx)
	insertPylovoWay(t, ctx, "res", "WAY-CODED", coded, `g`)
	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}

	byOSM, err := onrequest.BuildingsByOSMIDs(ctx, testPool, cfg, []string{"WAY-CODED"})
	if err != nil {
		t.Fatalf("BuildingsByOSMIDs: %v", err)
	}
	if got := functionCodeOf(t, byOSM, coded); got == nil || *got != want {
		t.Errorf("BuildingsByOSMIDs function_code of %s = %s, want %s", coded, show(got), want)
	}

	byBBox, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bboxAround(t, ctx, coded))
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	if got := functionCodeOf(t, byBBox, coded); got == nil || *got != want {
		t.Errorf("BuildingsByBBox function_code of %s = %s, want %s", coded, show(got), want)
	}

	byBBox, err = onrequest.BuildingsByBBox(ctx, testPool, cfg, bboxAround(t, ctx, uncoded))
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	if got := functionCodeOf(t, byBBox, uncoded); got != nil {
		t.Errorf("function_code of %s = %q, want null for a building without one", uncoded, *got)
	}

	mustExec(t, ctx, `DROP TABLE lod2.property CASCADE`)
	byBBox, err = onrequest.BuildingsByBBox(ctx, testPool, cfg, bboxAround(t, ctx, coded))
	if err != nil {
		t.Fatalf("BuildingsByBBox without lod2.property: %v", err)
	}
	if got := functionCodeOf(t, byBBox, coded); got != nil {
		t.Errorf("function_code without lod2.property = %q, want null", *got)
	}
}
