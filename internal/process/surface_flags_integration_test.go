//go:build integration

package process_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_SurfaceFlags pins is_valid and is_planar to the face in its own plane:
//
//	BOX     10 m box: vertical walls and a flat roof, all valid and planar
//	BOWTIE  a self-intersecting wall with unequal lobes (equal lobes cancel its normal): invalid
//	WARP2   a flat roof with one corner 2 mm high: planar within 0.01 m
//	WARP80  a flat roof with one corner 80 mm high: about 20 mm off its mid-plane, not planar
func TestPipeline_SurfaceFlags(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const y, z0, z1 = 342800.0, 200.0, 210.0
	f := &partsFixture{nextID: 800000}
	f.solid(f.feature(901, "BOX"), box{-4000, y, -3990, y + 10, z0, z1, "SENW", z0})

	bow := f.feature(901, "BOWTIE")
	f.solid(bow, box{-3970, y, -3960, y + 10, z0, z1, "SENW", z0})
	f.surface(bow, 709, [][3]float64{{-3970, y, z0}, {-3960, y, z1}, {-3960, y, z0}, {-3968, y, z0 + 6}})

	warped := func(id string, x, lift float64) {
		b := f.feature(901, id)
		fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), b)
		f.surface(b, 710, [][3]float64{{x, y, z0}, {x, y + 10, z0}, {x + 10, y + 10, z0}, {x + 10, y, z0}})
		f.surface(b, 712, [][3]float64{{x, y, z1}, {x + 10, y, z1}, {x + 10, y + 10, z1 + lift}, {x, y + 10, z1}})
		f.surface(b, 709, [][3]float64{{x, y, z0}, {x + 10, y, z0}, {x + 10, y, z1}, {x, y, z1}})
	}
	warped("WARP2", -3940, 0.002)
	warped("WARP80", -3910, 0.08)

	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	type flags struct{ faces, invalid, nonPlanar int }
	read := func(id, class string) flags {
		var g flags
		if err := testPool.QueryRow(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (WHERE NOT is_valid), COUNT(*) FILTER (WHERE NOT is_planar)
			FROM city2tabula.lod2_surface_raw WHERE building_object_id = $1 AND classname = $2`, id, class,
		).Scan(&g.faces, &g.invalid, &g.nonPlanar); err != nil {
			t.Fatalf("read %s %s flags: %v", id, class, err)
		}
		return g
	}
	for _, c := range []struct {
		id, class string
		want      flags
	}{
		{"BOX", "WallSurface", flags{4, 0, 0}},
		{"BOX", "RoofSurface", flags{1, 0, 0}},
		{"BOWTIE", "WallSurface", flags{5, 1, 0}},
		{"WARP2", "RoofSurface", flags{1, 0, 0}},
		{"WARP80", "RoofSurface", flags{1, 0, 1}},
	} {
		if got := read(c.id, c.class); got != c.want {
			t.Errorf("%s %s: got %+v, want %+v", c.id, c.class, got, c.want)
		}
	}
}
