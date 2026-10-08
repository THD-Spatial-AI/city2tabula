//go:build integration

package process_test

import (
	"context"
	"os"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// TestSurfaceBuilder_FlagsAreaBelowPrecision pins the contract for a surface whose
// area rounds away. The row must survive, because its geometry is source data
// and still renders, while area_below_precision tells a thermal consumer not to
// read the zero as a real area. Slivers from wall and roof intersections in the
// source model are the case this exists for.
func TestSurfaceBuilder_FlagsAreaBelowPrecision(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	// One sliver, one ordinary wall, distinguished only by area. The surface builder serves
	// surfaces only for buildings script 04 gave a row, so the building row is seeded too.
	seed := `
		INSERT INTO city2tabula.lod2_building (object_id, country_code, dataset_id, building_feature_id)
		VALUES ('bld-sliver', 'DE', 'test-dataset', 4242);
		INSERT INTO city2tabula.lod2_surface_raw
			(building_feature_id, surface_feature_id, building_object_id,
			 surface_object_id, objectclass_id, classname, surface_area,
			 surface_area_unit, tilt, tilt_unit, azimuth, azimuth_unit,
			 is_planar, is_party_wall, geom)
		VALUES
			(4242, 1, 'bld-sliver', 'srf-sliver', 709, 'WallSurface', 0.00,
			 'sqm', 0, 'degrees', 316.12, 'degrees', TRUE, FALSE,
			 ST_GeomFromText('POLYGON Z((0 0 0, 0 0.1 0, 0 0.1 0.05, 0 0 0))', 25832)),
			(4242, 2, 'bld-sliver', 'srf-normal', 709, 'WallSurface', 12.50,
			 'sqm', 0, 'degrees', 45.0, 'degrees', TRUE, FALSE,
			 ST_GeomFromText('POLYGON Z((1 0 0, 1 5 0, 1 5 2.5, 1 0 0))', 25832))`
	seedDB(t) // the dataset_attribution row the building's dataset_id references
	if _, err := testPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed lod2_surface_raw: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx,
			`DELETE FROM city2tabula.lod2_surface WHERE building_object_id = 'bld-sliver'`)
		_, _ = testPool.Exec(ctx,
			`DELETE FROM city2tabula.lod2_surface_raw WHERE building_object_id = 'bld-sliver'`)
		_, _ = testPool.Exec(ctx,
			`DELETE FROM city2tabula.lod2_building WHERE object_id = 'bld-sliver'`)
	})

	script, err := os.ReadFile("sql/scripts/post/03_build_surface.sql")
	if err != nil {
		t.Fatalf("read the surface builder: %v", err)
	}
	if _, err := testPool.Exec(ctx, applyParams(string(script))); err != nil {
		t.Fatalf("run the surface builder: %v", err)
	}

	rows := map[string]struct {
		area    float64
		flagged bool
	}{}
	cur, err := testPool.Query(ctx, `
		SELECT surface_object_id, surface_area, area_below_precision
		FROM city2tabula.lod2_surface WHERE building_object_id = 'bld-sliver'`)
	if err != nil {
		t.Fatalf("query lod2_surface: %v", err)
	}
	defer cur.Close()
	for cur.Next() {
		var id string
		var area float64
		var flagged bool
		if err := cur.Scan(&id, &area, &flagged); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows[id] = struct {
			area    float64
			flagged bool
		}{area, flagged}
	}

	// The sliver is kept, not dropped: its geometry is still source data.
	if len(rows) != 2 {
		t.Fatalf("expected both surfaces to survive the surface builder, got %d: %v", len(rows), rows)
	}
	if !rows["srf-sliver"].flagged {
		t.Error("sliver with area 0.00 was not flagged area_below_precision")
	}
	if rows["srf-normal"].flagged {
		t.Errorf("ordinary %.2f m2 wall was flagged area_below_precision", rows["srf-normal"].area)
	}
}
