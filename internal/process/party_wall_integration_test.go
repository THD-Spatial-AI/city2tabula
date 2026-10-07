//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_PartyWalls runs extraction over 10 m by 10 m boxes:
//
//	WEST | MID | EAST   a terrace, 10 m high, sharing 10 m by 10 m walls
//	TALL | LOW          a 15 m box against a 10 m box: the lower 10 m of TALL's
//	                    east wall is shared, the top 5 m stays exterior
//	ALONE               detached
//
// A shared piece is served as its own row with is_party_wall and the neighbour's
// object_id, and is left out of area_total_wall and surface_count_wall.
func TestPipeline_PartyWalls(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const x, y, z0 = -3000.0, 342800.0, 200.0
	f := &partsFixture{nextID: 900000}
	for _, b := range []struct {
		id     string
		dx, dy float64
		height float64
	}{
		{"WEST", 0, 0, 10}, {"MID", 10, 0, 10}, {"EAST", 20, 0, 10},
		{"TALL", 0, 30, 15}, {"LOW", 10, 30, 10},
		{"ALONE", 40, 0, 10},
	} {
		f.solid(f.feature(901, b.id), box{x + b.dx, y + b.dy, x + b.dx + 10, y + b.dy + 10, z0, z0 + b.height, "SENW", z0})
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	type walls struct {
		envelope, party float64
		count           int
		neighbour       string
	}
	for id, w := range map[string]walls{
		"WEST":  {300, 100, 3, "MID"},
		"MID":   {200, 200, 2, ""}, // two neighbours; checked below
		"EAST":  {300, 100, 3, "MID"},
		"TALL":  {500, 100, 4, "LOW"},
		"LOW":   {300, 100, 3, "TALL"},
		"ALONE": {400, 0, 4, ""},
	} {
		var g walls
		if err := testPool.QueryRow(ctx, `
			SELECT b.area_total_wall, b.area_party_wall, b.surface_count_wall,
			       COALESCE((SELECT string_agg(DISTINCT s.neighbour_object_id, ',')
			                 FROM city2tabula.lod2_surface s
			                 WHERE s.building_object_id = b.object_id AND s.is_party_wall), '')
			FROM city2tabula.lod2_building b WHERE b.object_id = $1`, id,
		).Scan(&g.envelope, &g.party, &g.count, &g.neighbour); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if id == "MID" {
			w.neighbour = "EAST,WEST"
		}
		if math.Abs(g.envelope-w.envelope) > 0.05 || math.Abs(g.party-w.party) > 0.05 ||
			g.count != w.count || g.neighbour != w.neighbour {
			t.Errorf("%s: got %+v, want %+v", id, g, w)
		}
	}

	// TALL's east wall is served as two pieces: the shared lower 10 m and the
	// exterior top 5 m.
	var shared, exterior float64
	if err := testPool.QueryRow(ctx, `
		SELECT COALESCE(SUM(surface_area) FILTER (WHERE is_party_wall), 0),
		       COALESCE(SUM(surface_area) FILTER (WHERE NOT is_party_wall), 0)
		FROM city2tabula.lod2_surface
		WHERE building_object_id = 'TALL' AND surface_type = 'WallSurface' AND azimuth BETWEEN 80 AND 100`,
	).Scan(&shared, &exterior); err != nil {
		t.Fatalf("read TALL east wall: %v", err)
	}
	if math.Abs(shared-100) > 0.05 || math.Abs(exterior-50) > 0.05 {
		t.Errorf("TALL east wall: shared %.2f m2, exterior %.2f m2; want 100 and 50", shared, exterior)
	}

	// The geometry endpoint carries the flag, so a viewer can highlight the pieces.
	geometry, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{"MID"}, true)
	if err != nil {
		t.Fatalf("BuildingGeometryByObjectIDs: %v", err)
	}
	party := 0
	for _, g := range geometry {
		for _, s := range g.Surfaces {
			if s.IsPartyWall != nil && *s.IsPartyWall && s.NeighbourObjectID != nil && s.GeoJSON != nil {
				party++
			}
		}
	}
	if party != 2 {
		t.Errorf("geometry endpoint: MID has %d party-wall pieces with a neighbour and geometry, want 2", party)
	}
}
