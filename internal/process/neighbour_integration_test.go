//go:build integration

package process_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
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

// TestPipeline_AttachedNeighboursIgnoreImportOrder builds two pairs of a 4 m box
// lying inside a 10 m box, one pair imported large box first and the other small
// box first. Neither pair shares a wall, so whatever the import order, no
// building of either pair is attached.
func TestPipeline_AttachedNeighboursIgnoreImportOrder(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	f := &partsFixture{nextID: 300000}
	const x, y = -9000.0, 342800.0
	large := func(id string, dx float64) {
		f.solid(f.feature(901, id), box{x + dx, y, x + dx + 10, y + 10, 200, 210, "SENW", 200})
	}
	small := func(id string, dx float64) {
		f.solid(f.feature(901, id), box{x + dx + 3, y + 3, x + dx + 7, y + 7, 200, 203, "SENW", 200})
	}
	large("LARGE_FIRST", 0)
	small("SMALL_SECOND", 0)
	small("SMALL_FIRST", 50)
	large("LARGE_SECOND", 50)
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	mustExec(t, ctx, `
		INSERT INTO city2tabula.tabula_variant (tabula_variant_code_id, tabula_variant_code, max_volume,
			footprint_area, number_of_storeys, footprint_complexity, attached_neighbour_class, roof_complexity,
			area_total_roof, area_total_wall, area_total_floor)
		VALUES (1, 'V_ALONE', 1000, 100, 4, 0, 0, 0, 100, 400, 400)`)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	rows, err := testPool.Query(ctx, `
		SELECT object_id, attached_neighbour_class, attached_neighbour_id
		FROM city2tabula.lod2_building WHERE object_id = ANY($1) ORDER BY object_id`,
		[]string{"LARGE_FIRST", "SMALL_SECOND", "SMALL_FIRST", "LARGE_SECOND"})
	if err != nil {
		t.Fatalf("query lod2_building: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id string
		var class *int
		var ids []string
		if err := rows.Scan(&id, &class, &ids); err != nil {
			t.Fatalf("scan lod2_building: %v", err)
		}
		seen++
		if class == nil || *class != 0 || len(ids) != 0 {
			t.Errorf("%s: attached_neighbour_class %s, neighbours %v; want 0 and none", id, showInt(class), ids)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lod2_building: %v", err)
	}
	if seen != 4 {
		t.Errorf("found %d of the 4 fixture buildings", seen)
	}
}

// showInt prints a nullable int for a test message.
func showInt(v *int) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprint(*v)
}

// TestPipeline_ExtractionInArea extracts the terrace WEST | MID | EAST and a
// building FAR 1 km away area by area, as on-request runs do, and checks:
//   - an area holding WEST alone extracts WEST and MID, attached to it, so WEST's
//     neighbour class is final;
//   - post-processing leaves FAR, outside that run, as it was;
//   - after every area has run, each building matches one whole-table run.
func TestPipeline_ExtractionInArea(t *testing.T) {
	ctx := context.Background()
	cfg := areaFixture(t)
	runArea(t, ctx, cfg, 1000)
	// Markers a whole-table post-processing pass would overwrite.
	mustExec(t, ctx, `
		UPDATE city2tabula.lod2_building SET tabula_variant_code = 'UNTOUCHED' WHERE object_id = 'FAR';
		DELETE FROM city2tabula.lod2_surface WHERE building_object_id = 'FAR';`)

	runArea(t, ctx, cfg, 0)
	if got := neighbourClasses(t, ctx); fmt.Sprint(got) != "map[FAR:0 MID:1 WEST:1]" {
		t.Fatalf("after the WEST area got classes %v, want FAR:0 MID:1 WEST:1", got)
	}
	var label string
	var surfaces int
	if err := testPool.QueryRow(ctx, `
		SELECT tabula_variant_code, (SELECT count(*) FROM city2tabula.lod2_surface WHERE building_object_id = 'FAR')
		FROM city2tabula.lod2_building WHERE object_id = 'FAR'`).Scan(&label, &surfaces); err != nil {
		t.Fatalf("read FAR: %v", err)
	}
	if label != "UNTOUCHED" || surfaces != 0 {
		t.Fatalf("the WEST area run changed FAR: label %q, %d surfaces", label, surfaces)
	}

	runArea(t, ctx, cfg, 20)
	byArea := terraceState(t, ctx)

	cfg = areaFixture(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}
	if whole := terraceState(t, ctx); fmt.Sprint(byArea) != fmt.Sprint(whole) {
		t.Fatalf("area by area got\n%v\nwant, as one whole-table run gives,\n%v", byArea, whole)
	}
}

// areaTerraceX and areaTerraceY place the area fixture in the Vienna range of EPSG:31256.
const areaTerraceX, areaTerraceY = -9000.0, 342800.0

// areaFixture resets the schemas and seeds 10 m boxes WEST, MID, EAST, sharing
// walls, and FAR, 1 km east, with the envelopes citydb-tool fills on import.
func areaFixture(t *testing.T) *config.Config {
	t.Helper()
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	f := &partsFixture{nextID: 310000}
	f.sql.WriteString("ALTER TABLE lod2.feature ADD COLUMN envelope geometry(GeometryZ, 31256);\n")
	x, y := areaTerraceX, areaTerraceY
	for _, b := range []struct {
		id string
		dx float64
	}{{"WEST", 0}, {"MID", 10}, {"EAST", 20}, {"FAR", 1000}} {
		f.solid(f.feature(901, b.id), box{x + b.dx, y, x + b.dx + 10, y + 10, 200, 210, "SENW", 200})
		fmt.Fprintf(&f.sql, "UPDATE lod2.feature SET envelope = ST_Force3D(ST_MakeEnvelope(%f, %f, %f, %f, 31256)) WHERE objectid = '%s';\n",
			x+b.dx, y, x+b.dx+10, y+10, b.id)
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	// The parts fixture's own buildings have no envelope, so no area reaches them.
	seedDB(t)
	return cfg
}

// runArea runs extraction for a 6 m square inside the box dx metres east of WEST.
func runArea(t *testing.T, ctx context.Context, cfg *config.Config, dx float64) {
	t.Helper()
	x, y := areaTerraceX+dx, areaTerraceY
	var area process.Area
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, 31256), 4326) AS e) s`,
		x+2, y+2, x+8, y+8).Scan(&area.Xmin, &area.Ymin, &area.Xmax, &area.Ymax); err != nil {
		t.Fatalf("transform area: %v", err)
	}
	if err := process.RunFeatureExtractionInArea(cfg, testPool, area); err != nil {
		t.Fatalf("RunFeatureExtractionInArea at dx %v: %v", dx, err)
	}
}

// terraceState returns what post-processing writes for WEST, MID and EAST.
func terraceState(t *testing.T, ctx context.Context) []string {
	t.Helper()
	rows, err := testPool.Query(ctx, `
		SELECT concat_ws(' ', b.object_id, b.attached_neighbour_class, b.attached_neighbour_id::text,
		       b.area_party_wall, b.area_total_wall, b.surface_count_wall, b.tabula_variant_code,
		       (SELECT count(*) FROM city2tabula.lod2_surface s WHERE s.building_object_id = b.object_id),
		       (SELECT count(*) FROM city2tabula.lod2_surface s WHERE s.building_object_id = b.object_id AND s.is_party_wall))
		FROM city2tabula.lod2_building b
		WHERE b.object_id IN ('WEST', 'MID', 'EAST')
		ORDER BY b.object_id`)
	if err != nil {
		t.Fatalf("query terrace: %v", err)
	}
	defer rows.Close()
	var state []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan terrace: %v", err)
		}
		state = append(state, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read terrace: %v", err)
	}
	if len(state) != 3 {
		t.Fatalf("got %d terrace buildings, want 3: %v", len(state), state)
	}
	return state
}

// neighbourClasses returns the attached_neighbour_class of every extracted building, by object_id.
func neighbourClasses(t *testing.T, ctx context.Context) map[string]int {
	t.Helper()
	rows, err := testPool.Query(ctx, `SELECT object_id, attached_neighbour_class FROM city2tabula.lod2_building`)
	if err != nil {
		t.Fatalf("query lod2_building: %v", err)
	}
	defer rows.Close()
	classes := map[string]int{}
	for rows.Next() {
		var id string
		var class int
		if err := rows.Scan(&id, &class); err != nil {
			t.Fatalf("scan lod2_building: %v", err)
		}
		classes[id] = class
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read lod2_building: %v", err)
	}
	return classes
}
