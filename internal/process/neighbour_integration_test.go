//go:build integration

package process_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
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
// follows its class. The neighbours are listed and served by object_id.
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
		ids          []string
	}{
		"WEST": {1, 1, "V_END", []string{"MID"}}, "MID": {2, 2, "V_MID", []string{"EAST", "WEST"}},
		"EAST":   {1, 1, "V_END", []string{"MID"}},
		"CORNER": {0, 0, "V_ALONE", []string{}}, "ALONE": {0, 0, "V_ALONE", []string{}},
	}
	rows, err := testPool.Query(ctx, `
		SELECT object_id, has_attached_neighbour, attached_neighbour_class, total_attached_neighbour,
		       COALESCE(tabula_variant_code, ''), attached_neighbour_id
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
		var ids []string
		if err := rows.Scan(&id, &attached, &class, &total, &variant, &ids); err != nil {
			t.Fatalf("scan lod2_building: %v", err)
		}
		seen++
		w := want[id]
		if attached == nil || class == nil || total == nil {
			t.Errorf("%s: has_attached_neighbour, attached_neighbour_class or total_attached_neighbour is NULL", id)
			continue
		}
		if *attached != (w.total > 0) || *class != w.class || *total != w.total || variant != w.variant ||
			!slices.Equal(ids, w.ids) {
			t.Errorf("%s: attached %v, class %d, total %d, variant %q, neighbours %v; want class %d, total %d, variant %q, neighbours %v",
				id, *attached, *class, *total, variant, ids, w.class, w.total, w.variant, w.ids)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lod2_building: %v", err)
	}
	if seen != len(want) {
		t.Errorf("found %d of the %d fixture buildings", seen, len(want))
	}

	var bbox onrequest.Bbox
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, $5), 4326) AS e) s`,
		x, y, x+40, y+25, partsSRID,
	).Scan(&bbox.Xmin, &bbox.Ymin, &bbox.Xmax, &bbox.Ymax); err != nil {
		t.Fatalf("build bbox: %v", err)
	}
	served, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bbox)
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	for _, b := range served {
		if b.ObjectID != "MID" {
			continue
		}
		if b.HasAttachedNeighbour == nil || !*b.HasAttachedNeighbour || b.AttachedNeighbourClass == nil ||
			*b.AttachedNeighbourClass != 2 || !slices.Equal(b.AttachedNeighbourIDs, []string{"EAST", "WEST"}) {
			t.Errorf("served MID: attached %v, class %v, neighbours %v; want true, 2, [EAST WEST]",
				b.HasAttachedNeighbour, b.AttachedNeighbourClass, b.AttachedNeighbourIDs)
		}
		return
	}
	t.Errorf("BuildingsByBBox did not serve MID; got %d buildings", len(served))
}
