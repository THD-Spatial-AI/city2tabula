//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// partsFixture writes a minimal 3DCityDB lod2 schema (the four tables script 01
// reads) holding one building per source pattern for how BuildingParts are used:
//
//	PRAGUE  Building owns a 10 m box, its part owns an adjacent 20 m box; the wall
//	        between them is omitted on both sides and only the part's exposed strip
//	        above the Building's roof is modelled. An installation sits on the roof.
//	VIENNA  Building owns no solid; the same two boxes are closed part solids, so
//	        the wall between them is modelled on both sides.
//	BAG     Building owns no solid; exactly one part (id "BAG-0") owns a 10 m box.
//	BREMEN  Building owns a 10 m box and has no parts.
//	UNOTCH  Building owns no solid; one part is U-shaped and a second part fills
//	        its notch. Script 03's outward flip, which works from one interior point
//	        per solid, turns one of the notch walls inward, so the walls between the
//	        two parts cannot be told apart by the sign of their normals.
//
// PRAGUE and VIENNA describe the same envelope, so they must aggregate identically.
type partsFixture struct {
	nextID int64
	sql    strings.Builder
}

const partsSRID = 31256

// box is an axis-aligned block; walls lists which of S, E, N, W to emit, and
// westFrom raises the bottom of the west wall to model a partly omitted wall.
type box struct {
	x0, y0, x1, y1, z0, z1 float64
	walls                  string
	westFrom               float64
}

func (f *partsFixture) id() int64 { f.nextID++; return f.nextID }

func (f *partsFixture) feature(class int, objectID string) int64 {
	id := f.id()
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.feature (id, objectclass_id, objectid) VALUES (%d, %d, '%s');\n", id, class, objectID)
	return id
}

func (f *partsFixture) link(from int64, name string, to int64) {
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_feature_id) VALUES (%d, %d, '%s', %d);\n", f.id(), from, name, to)
}

// surface adds a thematic surface with one polygon ring (x y z triples) and
// attaches it to owner through the boundary property.
func (f *partsFixture) surface(owner int64, class int, ring [][3]float64) {
	sid := f.feature(class, fmt.Sprintf("S%d", f.nextID+1))
	pts := make([]string, 0, len(ring)+1)
	for _, p := range append(ring, ring[0]) {
		pts = append(pts, fmt.Sprintf("%g %g %g", p[0], p[1], p[2]))
	}
	gid := f.id()
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.geometry_data (id, geometry) VALUES (%d, ST_GeomFromText('MULTIPOLYGON Z (((%s)))', %d));\n",
		gid, strings.Join(pts, ", "), partsSRID)
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod, val_geometry_id) VALUES (%d, %d, 'lod2MultiSurface', '2', %d);\n", f.id(), sid, gid)
	f.link(owner, "boundary", sid)
}

// solid gives owner an lod2Solid property and the boundary surfaces of b, each
// ring wound counter-clockwise seen from outside.
func (f *partsFixture) solid(owner int64, b box) {
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), owner)
	f.surface(owner, 710, [][3]float64{{b.x0, b.y0, b.z0}, {b.x0, b.y1, b.z0}, {b.x1, b.y1, b.z0}, {b.x1, b.y0, b.z0}})
	f.surface(owner, 712, [][3]float64{{b.x0, b.y0, b.z1}, {b.x1, b.y0, b.z1}, {b.x1, b.y1, b.z1}, {b.x0, b.y1, b.z1}})
	walls := map[byte][][3]float64{
		'S': {{b.x0, b.y0, b.z0}, {b.x1, b.y0, b.z0}, {b.x1, b.y0, b.z1}, {b.x0, b.y0, b.z1}},
		'E': {{b.x1, b.y0, b.z0}, {b.x1, b.y1, b.z0}, {b.x1, b.y1, b.z1}, {b.x1, b.y0, b.z1}},
		'N': {{b.x1, b.y1, b.z0}, {b.x0, b.y1, b.z0}, {b.x0, b.y1, b.z1}, {b.x1, b.y1, b.z1}},
		'W': {{b.x0, b.y1, b.westFrom}, {b.x0, b.y0, b.westFrom}, {b.x0, b.y0, b.z1}, {b.x0, b.y1, b.z1}},
	}
	for i := 0; i < len(b.walls); i++ {
		f.surface(owner, 709, walls[b.walls[i]])
	}
}

// prism gives owner an lod2Solid property and the boundary surfaces of a vertical
// prism over footprint (x y pairs, counter-clockwise seen from above).
func (f *partsFixture) prism(owner int64, footprint [][2]float64, z0, z1 float64) {
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), owner)
	n := len(footprint)
	ground := make([][3]float64, n)
	roof := make([][3]float64, n)
	for i, p := range footprint {
		ground[n-1-i] = [3]float64{p[0], p[1], z0}
		roof[i] = [3]float64{p[0], p[1], z1}
	}
	f.surface(owner, 710, ground)
	f.surface(owner, 712, roof)
	for i, p := range footprint {
		q := footprint[(i+1)%n]
		f.surface(owner, 709, [][3]float64{{p[0], p[1], z0}, {q[0], q[1], z0}, {q[0], q[1], z1}, {p[0], p[1], z1}})
	}
}

func buildPartsFixture() string {
	f := &partsFixture{}
	f.sql.WriteString(`
CREATE SCHEMA lod2;
CREATE TABLE lod2.objectclass (id INTEGER PRIMARY KEY, classname TEXT);
CREATE TABLE lod2.feature (id BIGINT PRIMARY KEY, objectclass_id INTEGER NOT NULL, objectid TEXT, lineage TEXT);
CREATE TABLE lod2.geometry_data (id BIGINT PRIMARY KEY, geometry geometry(GeometryZ, 31256));
CREATE TABLE lod2.property (id BIGINT PRIMARY KEY, feature_id BIGINT, name TEXT, val_lod TEXT,
    val_geometry_id BIGINT, val_feature_id BIGINT);
INSERT INTO lod2.objectclass VALUES (709, 'WallSurface'), (710, 'GroundSurface'), (712, 'RoofSurface'),
    (901, 'Building'), (902, 'BuildingPart'), (905, 'BuildingInstallation');
`)
	// Coordinates sit in the Vienna range of EPSG:31256 so the plane arithmetic
	// runs at the magnitude real data has.
	const x, y = -10500.0, 342800.0
	low := func(dx float64, walls string) box {
		return box{x + dx, y, x + dx + 10, y + 10, 200, 210, walls, 200}
	}
	high := func(dx float64, walls string, westFrom float64) box {
		return box{x + dx + 10, y, x + dx + 20, y + 10, 200, 220, walls, westFrom}
	}

	prague := f.feature(901, "PRAGUE")
	f.solid(prague, low(0, "SNW"))
	praguePart := f.feature(902, "ID_generated-at-import")
	f.link(prague, "buildingPart", praguePart)
	f.solid(praguePart, high(0, "SENW", 210))
	inst := f.feature(905, "PRAGUE-dormer")
	f.link(prague, "buildingInstallation", inst)
	f.surface(inst, 712, [][3]float64{{x + 2, y + 2, 211}, {x + 4, y + 2, 211}, {x + 4, y + 4, 211}, {x + 2, y + 4, 211}})

	vienna := f.feature(901, "VIENNA")
	for i, b := range []box{low(100, "SENW"), high(100, "SENW", 200)} {
		part := f.feature(902, fmt.Sprintf("VIENNA_%d", i+1))
		f.link(vienna, "buildingPart", part)
		f.solid(part, b)
	}

	bag := f.feature(901, "BAG")
	bagPart := f.feature(902, "BAG-0")
	f.link(bag, "buildingPart", bagPart)
	f.solid(bagPart, low(200, "SENW"))

	bremen := f.feature(901, "BREMEN")
	f.solid(bremen, low(300, "SENW"))

	unotch := f.feature(901, "UNOTCH")
	u := f.feature(902, "UNOTCH_1")
	f.link(unotch, "buildingPart", u)
	ux := x + 400
	f.prism(u, [][2]float64{{ux, y}, {ux + 30, y}, {ux + 30, y + 30}, {ux + 20, y + 30},
		{ux + 20, y + 12}, {ux + 10, y + 12}, {ux + 10, y + 30}, {ux, y + 30}}, 200, 210)
	fill := f.feature(902, "UNOTCH_2")
	f.link(unotch, "buildingPart", fill)
	f.prism(fill, [][2]float64{{ux + 10, y + 12}, {ux + 20, y + 12}, {ux + 20, y + 30}, {ux + 10, y + 30}}, 200, 210)

	return f.sql.String()
}

// TestPipeline_BuildingParts_OneRowPerBuilding runs extraction over every
// BuildingPart pattern and checks that each CityGML Building yields one row keyed
// by its own object_id, with walls between its parts excluded and heights
// weighted by each part's footprint, so volume and floor area equal the sums
// over its parts.
func TestPipeline_BuildingParts_OneRowPerBuilding(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	tc := pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)}
	cfg := pipelineConfig(tc)
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	mustExec(t, ctx, buildPartsFixture())
	seedDB(t)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	type row struct {
		footprint, wall, roof, floor, minH, maxH, minV, maxV float64
		storeys, wallCount                                   int
	}
	want := map[string]row{
		"PRAGUE": {200, 1000, 200, 1200, 15, 15, 3000, 3000, 6, 7},
		"VIENNA": {200, 1000, 200, 1200, 15, 15, 3000, 3000, 6, 7},
		"BAG":    {100, 400, 100, 400, 10, 10, 1000, 1000, 4, 4},
		"BREMEN": {100, 400, 100, 400, 10, 10, 1000, 1000, 4, 4},
		"UNOTCH": {900, 1200, 900, 3600, 10, 10, 9000, 9000, 4, 6},
	}

	rows, err := testPool.Query(ctx, `
		SELECT object_id, footprint_area, area_total_wall, area_total_roof, area_total_floor,
		       min_height, max_height, min_volume, max_volume, number_of_storeys, surface_count_wall
		FROM city2tabula.lod2_building`)
	if err != nil {
		t.Fatalf("query lod2_building: %v", err)
	}
	got := map[string]row{}
	for rows.Next() {
		var id string
		var r row
		if err := rows.Scan(&id, &r.footprint, &r.wall, &r.roof, &r.floor, &r.minH, &r.maxH, &r.minV, &r.maxV, &r.storeys, &r.wallCount); err != nil {
			t.Fatalf("scan lod2_building: %v", err)
		}
		got[id] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate lod2_building: %v", err)
	}

	if len(got) != len(want) {
		t.Errorf("expected %d building rows (one per CityGML Building), got %d: %v", len(want), len(got), got)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.05 }
	for id, w := range want {
		g, ok := got[id]
		if !ok {
			t.Errorf("no lod2_building row with object_id %q", id)
			continue
		}
		if !near(g.footprint, w.footprint) || !near(g.wall, w.wall) || !near(g.roof, w.roof) || !near(g.floor, w.floor) ||
			!near(g.minH, w.minH) || !near(g.maxH, w.maxH) || !near(g.minV, w.minV) || !near(g.maxV, w.maxV) ||
			g.storeys != w.storeys || g.wallCount != w.wallCount {
			t.Errorf("%s: got %+v, want %+v", id, g, w)
		}
	}

	// The served surfaces must describe the same envelope the building row reports:
	// VIENNA's fully internal wall dropped, its half-internal wall clipped.
	for _, id := range []string{"PRAGUE", "VIENNA"} {
		var n int
		var area float64
		if err := testPool.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(surface_area), 0) FROM city2tabula.lod2_surface
			WHERE building_object_id = $1 AND surface_type = 'WallSurface'`, id,
		).Scan(&n, &area); err != nil {
			t.Fatalf("query lod2_surface for %s: %v", id, err)
		}
		if n != 7 || !near(area, 1000) {
			t.Errorf("%s: lod2_surface walls = %d faces / %.2f m2, want 7 / 1000", id, n, area)
		}
	}
}

// TestPipeline_GroundOnlyBuilding_Dropped adds a building whose only part carries a
// single GroundSurface, the shape a converter emits when it loses a segment's walls
// and roof. It has no envelope to classify or serve, so extraction must give it no
// lod2_building row and no lod2_surface rows, and leave the other buildings intact.
func TestPipeline_GroundOnlyBuilding_Dropped(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	tc := pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)}
	cfg := pipelineConfig(tc)
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	f := &partsFixture{nextID: 100000}
	const x, y = -10000.0, 342800.0
	flat := f.feature(901, "GROUNDONLY")
	part := f.feature(902, "GROUNDONLY-1")
	f.link(flat, "buildingPart", part)
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), part)
	f.surface(part, 710, [][3]float64{{x, y, 200}, {x, y + 10, 200}, {x + 10, y + 10, 200}, {x + 10, y, 200}})
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var buildings, groundOnly, surfaces int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE object_id = 'GROUNDONLY'),
		       (SELECT COUNT(*) FROM city2tabula.lod2_surface WHERE building_object_id = 'GROUNDONLY')
		FROM city2tabula.lod2_building`,
	).Scan(&buildings, &groundOnly, &surfaces); err != nil {
		t.Fatalf("query lod2_building: %v", err)
	}
	if groundOnly != 0 || surfaces != 0 {
		t.Errorf("GROUNDONLY: got %d lod2_building rows and %d lod2_surface rows, want 0 and 0", groundOnly, surfaces)
	}
	if buildings != 5 {
		t.Errorf("expected the 5 buildings with walls and roofs to remain, got %d rows", buildings)
	}
}
