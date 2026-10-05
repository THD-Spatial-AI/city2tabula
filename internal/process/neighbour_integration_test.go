//go:build integration

package process_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_AttachedNeighbours runs extraction over identical 10 m boxes:
//
//	WEST | MID | EAST   a terrace sharing 10 m walls
//	CORNER              touches EAST's north-east corner only
//	ALONE               5 m from everything
//
// With one building per batch, every pair crosses a batch boundary. Three TABULA
// variants differ only in attached_neighbour_class, so each building's label
// follows its class.
func TestPipeline_AttachedNeighbours(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	cfg.Batch.Size = 1
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	f := &partsFixture{nextID: 300000}
	const x, y = -9000.0, 342800.0
	for _, b := range []struct {
		id     string
		dx, dy float64
	}{
		{"WEST", 0, 0}, {"MID", 10, 0}, {"EAST", 20, 0}, {"CORNER", 30, 10}, {"ALONE", 0, 15},
	} {
		f.solid(f.feature(901, b.id), box{x + b.dx, y + b.dy, x + b.dx + 10, y + b.dy + 10, 200, 210, "SENW", 200})
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	mustExec(t, ctx, `
		INSERT INTO city2tabula.tabula_variant (tabula_variant_code_id, tabula_variant_code, max_volume,
			footprint_area, number_of_storeys, footprint_complexity, attached_neighbour_class, roof_complexity,
			area_total_roof, area_total_wall, area_total_floor)
		VALUES (1, 'V_ALONE', 1000, 100, 4, 0, 0, 0, 100, 400, 400),
		       (2, 'V_END',   1000, 100, 4, 0, 1, 0, 100, 400, 400),
		       (3, 'V_MID',   1000, 100, 4, 0, 2, 0, 100, 400, 400)`)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	want := map[string]struct {
		class, total int
		variant      string
	}{
		"WEST": {1, 1, "V_END"}, "MID": {2, 2, "V_MID"}, "EAST": {1, 1, "V_END"},
		"CORNER": {0, 0, "V_ALONE"}, "ALONE": {0, 0, "V_ALONE"},
	}
	rows, err := testPool.Query(ctx, `
		SELECT object_id, has_attached_neighbour, attached_neighbour_class, total_attached_neighbour,
		       COALESCE(tabula_variant_code, '')
		FROM city2tabula.lod2_building
		WHERE object_id = ANY($1)`, []string{"WEST", "MID", "EAST", "CORNER", "ALONE"})
	if err != nil {
		t.Fatalf("query lod2_building: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id, variant string
		var attached *bool
		var class, total *int
		if err := rows.Scan(&id, &attached, &class, &total, &variant); err != nil {
			t.Fatalf("scan lod2_building: %v", err)
		}
		seen++
		w := want[id]
		if attached == nil || class == nil || total == nil {
			t.Errorf("%s: has_attached_neighbour, attached_neighbour_class or total_attached_neighbour is NULL", id)
			continue
		}
		if *attached != (w.total > 0) || *class != w.class || *total != w.total || variant != w.variant {
			t.Errorf("%s: attached %v, class %d, total %d, variant %q; want class %d, total %d, variant %q",
				id, *attached, *class, *total, variant, w.class, w.total, w.variant)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lod2_building: %v", err)
	}
	if seen != len(want) {
		t.Errorf("found %d of the %d fixture buildings", seen, len(want))
	}
}
