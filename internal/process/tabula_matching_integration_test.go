//go:build integration

package process_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_TabulaMatching_UnknownValuesAndIntegerCodes pins two properties of the
// TABULA match, each with variants that differ in one dimension only:
//
//   - BOX (10 m cube, 4 storeys): V_PART matches it on every value it has but its
//     volume is unknown; V_FULL is known everywhere but five times the volume. An
//     unknown value drops out of the distance, so V_PART wins.
//   - TALL (16.8 m high, 6 storeys of 2.8 m): V_ONE and V_SEVEN differ only in storeys. Storeys
//     are integers and must be normalised without integer division, so 6 lies
//     nearer 7 than 1.
func TestPipeline_TabulaMatching_UnknownValuesAndIntegerCodes(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const x, y = -7000.0, 342800.0
	f := &partsFixture{nextID: 600000}
	f.solid(f.feature(901, "BOX"), box{x, y, x + 10, y + 10, 200, 210, "SENW", 200})
	f.solid(f.feature(901, "TALL"), box{x + 50, y, x + 60, y + 10, 200, 216.8, "SENW", 200})
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	mustExec(t, ctx, `
		INSERT INTO city2tabula.tabula_variant (tabula_variant_code_id, tabula_variant_code, max_volume,
			footprint_area, number_of_storeys, area_total_roof, area_total_wall, area_total_floor)
		VALUES (1, 'V_FULL',  5000, 100, 4, 100, 400, 400),
		       (2, 'V_PART',  NULL, 100, 4, 100, 400, 400),
		       (3, 'V_ONE',   1680, 100, 1, 100, 672, 600),
		       (4, 'V_SEVEN', 1680, 100, 7, 100, 672, 600)`)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	for id, want := range map[string]string{"BOX": "V_PART", "TALL": "V_SEVEN"} {
		var storeys int
		var got string
		if err := testPool.QueryRow(ctx, `
			SELECT number_of_storeys, COALESCE(tabula_variant_code, '')
			FROM city2tabula.lod2_building WHERE object_id = $1`, id,
		).Scan(&storeys, &got); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if got != want {
			t.Errorf("%s (%d storeys): matched %q, want %q", id, storeys, got, want)
		}
	}
}
