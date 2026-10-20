//go:build integration

package process_test

import (
	"context"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestServe_BuildingsByObjectIDs pins the object_ids mode: every requested building is
// returned once with its envelope, linked or not. A building linked to two ways carries
// both, best first, with match_type 1; one linked without an OSM match has empty ways and
// match_type 2; one without a link row has empty ways and match_type 0; an unknown id
// returns nothing.
func TestServe_BuildingsByObjectIDs(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	linked, unmatched, unlinked := pylovoLinkFixtureBuildings(t, ctx)
	seedPylovoTestTables(t, ctx)
	insertPylovoWay(t, ctx, "res", "WAY-A", linked, `g`)
	insertPylovoWay(t, ctx, "oth", "WAY-B", linked, `ST_Intersection(g, ST_MakeEnvelope(xmin, ymin, xmin + 0.6 * w, ymax, ST_SRID(g)))`)
	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}
	mustExec(t, ctx, `DELETE FROM city2tabula.building_link WHERE object_id = '`+unlinked+`'`)

	got, err := onrequest.BuildingsByObjectIDs(ctx, testPool, cfg, []string{linked, unmatched, unlinked, "NO-SUCH-BUILDING"})
	if err != nil {
		t.Fatalf("BuildingsByObjectIDs: %v", err)
	}
	byID := map[string]onrequest.Building{}
	for _, b := range got {
		if _, dup := byID[b.ObjectID]; dup {
			t.Errorf("%s returned twice", b.ObjectID)
		}
		byID[b.ObjectID] = b
	}
	if len(byID) != 3 {
		t.Fatalf("returned %d buildings, want the 3 known ones", len(byID))
	}
	for _, c := range []struct {
		id        string
		osmID     string
		osmIDs    []string
		matchType int16
	}{
		{linked, "WAY-A", []string{"WAY-A", "WAY-B"}, 1},
		{unmatched, "", []string{}, 2},
		{unlinked, "", []string{}, 0},
	} {
		b := byID[c.id]
		if b.OSMID != c.osmID || !slices.Equal(b.OSMIDs, c.osmIDs) || b.MatchType != c.matchType {
			t.Errorf("%s: osm_id %q, osm_ids %v, match_type %d; want %q, %v, %d",
				c.id, b.OSMID, b.OSMIDs, b.MatchType, c.osmID, c.osmIDs, c.matchType)
		}
		if len(b.Surfaces) == 0 || b.FunctionCode == nil {
			t.Errorf("%s: %d surfaces, function_code %v; want its envelope and source function code", c.id, len(b.Surfaces), b.FunctionCode)
		}
	}
}
