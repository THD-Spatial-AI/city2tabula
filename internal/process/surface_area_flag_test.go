//go:build integration

package process_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// TestScript08_FlagsAreaBelowPrecision pins the contract for a surface whose
// area rounds away. The row must survive, because its geometry is source data
// and still renders, while area_below_precision tells a thermal consumer not to
// read the zero as a real area. Slivers from wall and roof intersections in the
// source model are the case this exists for.
func TestScript08_FlagsAreaBelowPrecision(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	// One sliver, one ordinary wall, distinguished only by area.
	seed := `
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
	if _, err := testPool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed lod2_surface_raw: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx,
			`DELETE FROM city2tabula.lod2_surface WHERE building_object_id = 'bld-sliver'`)
		_, _ = testPool.Exec(ctx,
			`DELETE FROM city2tabula.lod2_surface_raw WHERE building_object_id = 'bld-sliver'`)
	})

	script, err := os.ReadFile("sql/scripts/main/08_build_surface.sql")
	if err != nil {
		t.Fatalf("read script 08: %v", err)
	}
	sql := applyParams(string(script))
	// The seeded building, rather than the fixed ids the benchmarks use.
	sql = replaceBuildingIDs(sql, "(4242)")
	if _, err := testPool.Exec(ctx, sql); err != nil {
		t.Fatalf("run script 08: %v", err)
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
		t.Fatalf("expected both surfaces to survive script 08, got %d: %v", len(rows), rows)
	}
	if !rows["srf-sliver"].flagged {
		t.Error("sliver with area 0.00 was not flagged area_below_precision")
	}
	if rows["srf-normal"].flagged {
		t.Errorf("ordinary %.2f m2 wall was flagged area_below_precision", rows["srf-normal"].area)
	}
}

// replaceBuildingIDs swaps the {building_ids} value applyParams substituted for
// one naming the building this test seeds.
func replaceBuildingIDs(sql, ids string) string {
	return strings.ReplaceAll(sql, sqlParams["{building_ids}"], ids)
}
