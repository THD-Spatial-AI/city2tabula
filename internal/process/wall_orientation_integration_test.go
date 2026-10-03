//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_ConcaveFootprint_WallsFaceOutward pins the azimuth of every wall
// of an L-shaped building. Testing each wall against one interior point of the
// solid turns the walls of the inner corner inward, 180 degrees off; every wall
// must face away from its own footprint.
func TestPipeline_ConcaveFootprint_WallsFaceOutward(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	f := &partsFixture{nextID: 300000}
	const x, y = -9000.0, 342800.0
	bld := f.feature(901, "LSHAPE")
	f.prism(bld, [][2]float64{{x, y}, {x + 20, y}, {x + 20, y + 10}, {x + 10, y + 10}, {x + 10, y + 20}, {x, y + 20}}, 200, 210)
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	// Each wall by the centre of its footprint edge, relative to the L's corner,
	// and the bearing it faces: 0 north, 90 east, 180 south, 270 west.
	want := map[[2]int]float64{
		{10, 0}: 180, {20, 5}: 90, {15, 10}: 0, {10, 15}: 90, {5, 20}: 0, {0, 10}: 270,
	}
	rows, err := testPool.Query(ctx, `
		SELECT round(ST_X(ST_Centroid(ST_Force2D(geom))) - $1)::int,
		       round(ST_Y(ST_Centroid(ST_Force2D(geom))) - $2)::int, azimuth
		FROM city2tabula.lod2_surface_raw
		WHERE building_object_id = 'LSHAPE' AND classname = 'WallSurface'`, x, y)
	if err != nil {
		t.Fatalf("read walls: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var cx, cy int
		var azimuth float64
		if err := rows.Scan(&cx, &cy, &azimuth); err != nil {
			t.Fatalf("scan wall: %v", err)
		}
		seen++
		w, ok := want[[2]int{cx, cy}]
		if !ok {
			t.Errorf("unexpected wall at (%d, %d)", cx, cy)
			continue
		}
		if d := math.Mod(math.Abs(azimuth-w), 360); math.Min(d, 360-d) > 0.5 {
			t.Errorf("wall at (%d, %d): azimuth %.1f, want %.0f", cx, cy, azimuth, w)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate walls: %v", err)
	}
	if seen != len(want) {
		t.Errorf("got %d walls, want %d", seen, len(want))
	}
}
