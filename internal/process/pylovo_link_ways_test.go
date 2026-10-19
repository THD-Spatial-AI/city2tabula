//go:build integration

package process_test

import (
	"context"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// insertPylovoWay seeds a PyLovo row whose geometry is shape applied to a fixture
// building's footprint; shape is an SQL expression over the footprint g, its width w
// and its envelope corners xmin, ymin, xmax, ymax, all in the fixture's CRS.
func insertPylovoWay(t *testing.T, ctx context.Context, table, osmID, objectID, shape string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `
		INSERT INTO public.`+table+` (osm_id, country_code, geom)
		SELECT $1, country_code, ST_Multi(ST_Force2D(ST_Transform(`+shape+`, 3035)))
		FROM (
			SELECT country_code, ST_Force2D(building_footprint_geom) AS g,
			       ST_XMax(building_footprint_geom) - ST_XMin(building_footprint_geom) AS w,
			       ST_XMin(building_footprint_geom) AS xmin, ST_YMin(building_footprint_geom) AS ymin,
			       ST_XMax(building_footprint_geom) AS xmax, ST_YMax(building_footprint_geom) AS ymax
			FROM city2tabula.lod2_building WHERE object_id = $2
		) f`,
		osmID, objectID,
	); err != nil {
		t.Fatalf("seed pylovo.%s way %s for %s: %v", table, osmID, objectID, err)
	}
}

// TestRunPyLovoLinkBuild_LinksEveryWayAboveThreshold pins issue 203: a 3D building that
// OSM maps as several ways links every way whose overlap clears 0.5. With A60 and A20
// the areas of the footprint's west 60% and 20%:
//
//	WAY-FULL   the footprint                         confidence 1, the primary way
//	WAY-WEST   its west 60%                          confidence 1
//	WAY-SHIFT  west 60% plus a far copy of west 20%  confidence A60/(A60+A20), in [0.5, 1)
//	WAY-FAR    west 20% plus a far copy of the whole confidence A20/A, below 0.5
//
// The API returns the building once, under its primary way, with every linked way in
// osm_ids, ordered by confidence.
func TestRunPyLovoLinkBuild_LinksEveryWayAboveThreshold(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	seedPylovoTestTables(t, ctx)
	building, _, _ := pylovoLinkFixtureBuildings(t, ctx)
	west := func(share string) string {
		return `ST_Intersection(g, ST_MakeEnvelope(xmin, ymin, xmin + ` + share + ` * w, ymax, ST_SRID(g)))`
	}
	far := func(geom string) string { return `ST_Translate(` + geom + `, 3 * w, 0)` }
	insertPylovoWay(t, ctx, "res", "WAY-FULL", building, `g`)
	insertPylovoWay(t, ctx, "res", "WAY-WEST", building, west("0.6"))
	insertPylovoWay(t, ctx, "oth", "WAY-SHIFT", building, `ST_Union(`+west("0.6")+`, `+far(west("0.2"))+`)`)
	insertPylovoWay(t, ctx, "oth", "WAY-FAR", building, `ST_Union(`+west("0.2")+`, `+far(`g`)+`)`)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}

	rows, err := testPool.Query(ctx, `
		SELECT osm_id, match_type, match_confidence FROM city2tabula.building_link
		WHERE object_id = $1 ORDER BY match_type, match_confidence DESC`, building)
	if err != nil {
		t.Fatalf("read links of %s: %v", building, err)
	}
	type link struct {
		osmID      string
		matchType  int
		confidence float64
	}
	var links []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.osmID, &l.matchType, &l.confidence); err != nil {
			t.Fatalf("scan link: %v", err)
		}
		links = append(links, l)
	}
	rows.Close()

	if len(links) != 3 {
		t.Fatalf("links of %s = %+v, want WAY-FULL (1), WAY-WEST (4), WAY-SHIFT (4)", building, links)
	}
	if links[0].osmID != "WAY-FULL" || links[0].matchType != 1 {
		t.Errorf("primary link = %+v, want WAY-FULL with match_type 1", links[0])
	}
	if links[1].osmID != "WAY-WEST" || links[1].matchType != 4 || links[2].osmID != "WAY-SHIFT" || links[2].matchType != 4 {
		t.Errorf("further links = %+v, want WAY-WEST then WAY-SHIFT, both match_type 4", links[1:])
	}
	if c := links[2].confidence; c < 0.5 || c >= 1 {
		t.Errorf("WAY-SHIFT confidence = %v, want a partial overlap in [0.5, 1)", c)
	}

	wantWays := []string{"WAY-FULL", "WAY-WEST", "WAY-SHIFT"}
	for _, asked := range [][]string{{"WAY-SHIFT"}, {"WAY-FULL", "WAY-WEST"}} {
		got, err := onrequest.BuildingsByOSMIDs(ctx, testPool, cfg, asked)
		if err != nil {
			t.Fatalf("BuildingsByOSMIDs(%v): %v", asked, err)
		}
		if len(got) != 1 || got[0].ObjectID != building || got[0].OSMID != "WAY-FULL" || !slices.Equal(got[0].OSMIDs, wantWays) {
			t.Errorf("BuildingsByOSMIDs(%v) = %d buildings %+v, want %s once with osm_id WAY-FULL and osm_ids %v",
				asked, len(got), got, building, wantWays)
		}
	}
	if got, _ := onrequest.BuildingsByOSMIDs(ctx, testPool, cfg, []string{"WAY-FAR"}); len(got) != 0 {
		t.Errorf("BuildingsByOSMIDs(WAY-FAR) returned %d buildings, want none below the threshold", len(got))
	}
}

// TestRunPyLovoLinkBuild_UpdatesOldLinkTable pins that a building_link table created by
// an earlier release, with one row per 3D building and match_type limited to 1-3, is
// brought to the current shape by the link step instead of rejecting further ways.
func TestRunPyLovoLinkBuild_UpdatesOldLinkTable(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	mustExec(t, ctx, `
		DROP INDEX city2tabula.building_link_object_osm_country_key;
		ALTER TABLE city2tabula.building_link DROP CONSTRAINT building_link_match_type_check;
		ALTER TABLE city2tabula.building_link ADD CONSTRAINT building_link_match_type_check CHECK (match_type IN (1, 2, 3));
		ALTER TABLE city2tabula.building_link ADD CONSTRAINT building_link_object_id_country_code_key UNIQUE (object_id, country_code);`)

	seedPylovoTestTables(t, ctx)
	building, _, _ := pylovoLinkFixtureBuildings(t, ctx)
	insertPylovoWay(t, ctx, "res", "WAY-A", building, `g`)
	insertPylovoWay(t, ctx, "oth", "WAY-B", building, `g`)

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild on an old-shape table: %v", err)
	}
	var n int
	if err := testPool.QueryRow(ctx,
		`SELECT count(*) FROM city2tabula.building_link WHERE object_id = $1 AND match_type IN (1, 4)`, building,
	).Scan(&n); err != nil {
		t.Fatalf("count links of %s: %v", building, err)
	}
	if n != 2 {
		t.Errorf("%s has %d links after linking on an old-shape table, want 2 (WAY-A and WAY-B)", building, n)
	}
	// A second run finds the table current and changes nothing.
	if err := process.RunPyLovoRelink(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoRelink on an updated table: %v", err)
	}
}
