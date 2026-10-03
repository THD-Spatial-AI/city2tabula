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

// surfaceRings adds a thematic surface whose polygon has an outer ring and inner
// rings (x y z triples, each closed by repeating its first vertex here).
func (f *partsFixture) surfaceRings(owner int64, class int, rings ...[][3]float64) {
	sid := f.feature(class, fmt.Sprintf("S%d", f.nextID+1))
	wkt := make([]string, 0, len(rings))
	for _, ring := range rings {
		pts := make([]string, 0, len(ring)+1)
		for _, p := range append(ring, ring[0]) {
			pts = append(pts, fmt.Sprintf("%g %g %g", p[0], p[1], p[2]))
		}
		wkt = append(wkt, "("+strings.Join(pts, ", ")+")")
	}
	gid := f.id()
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.geometry_data (id, geometry) VALUES (%d, ST_GeomFromText('MULTIPOLYGON Z ((%s))', %d));\n",
		gid, strings.Join(wkt, ", "), partsSRID)
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod, val_geometry_id) VALUES (%d, %d, 'lod2MultiSurface', '2', %d);\n", f.id(), sid, gid)
	f.link(owner, "boundary", sid)
}

// TestPipeline_FaceWithInnerRing_AreaAndTilt pins the normal of a face with a
// hole. Its vertices must be paired within each ring: pairing a vertex of the
// outer ring with one of the hole gives a wrong normal, and the area measured
// in the plane of that normal is wrong with it.
func TestPipeline_FaceWithInnerRing_AreaAndTilt(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	// Near the origin a vertex pair taken across rings tilts the normal far off
	// vertical; at real projected coordinates the same error is masked on a flat
	// face by the size of the coordinates and shows on sloped ones.
	f := &partsFixture{nextID: 200000}
	const x, y, z0, z1 = 10.0, 10.0, 0.0, 10.0
	bld := f.feature(901, "HOLED")
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), bld)
	f.surface(bld, 710, [][3]float64{{x, y, z0}, {x, y + 10, z0}, {x + 10, y + 10, z0}, {x + 10, y, z0}})
	// A flat 10 m roof with a 4 m courtyard: outer ring counter-clockwise seen
	// from above, the hole clockwise.
	f.surfaceRings(bld, 712,
		[][3]float64{{x, y, z1}, {x + 10, y, z1}, {x + 10, y + 10, z1}, {x, y + 10, z1}},
		[][3]float64{{x + 3, y + 3, z1}, {x + 3, y + 7, z1}, {x + 7, y + 7, z1}, {x + 7, y + 3, z1}},
	)
	for _, w := range [][][3]float64{
		{{x, y, z0}, {x + 10, y, z0}, {x + 10, y, z1}, {x, y, z1}},
		{{x + 10, y, z0}, {x + 10, y + 10, z0}, {x + 10, y + 10, z1}, {x + 10, y, z1}},
		{{x + 10, y + 10, z0}, {x, y + 10, z0}, {x, y + 10, z1}, {x + 10, y + 10, z1}},
		{{x, y + 10, z0}, {x, y, z0}, {x, y, z1}, {x, y + 10, z1}},
	} {
		f.surface(bld, 709, w)
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var area, tilt float64
	if err := testPool.QueryRow(ctx, `
		SELECT surface_area, tilt FROM city2tabula.lod2_surface_raw
		WHERE building_object_id = 'HOLED' AND classname = 'RoofSurface'`,
	).Scan(&area, &tilt); err != nil {
		t.Fatalf("read the holed roof: %v", err)
	}
	if math.Abs(area-84) > 0.05 || math.Abs(tilt-90) > 0.05 {
		t.Errorf("holed roof: area %.2f m2, tilt %.2f; want 84.00 m2 and 90.00 (flat)", area, tilt)
	}
}
