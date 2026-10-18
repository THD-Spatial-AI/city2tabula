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

// TestPipeline_TabulaMatching_UnknownValuesAndIntegerCodes pins two properties of the
// TABULA match, each with variants that differ in one dimension only:
//
//   - BOX (10 m cube, 4 storeys): V_PART matches it on every value it has but its
//     volume is unknown; V_FULL is known everywhere but five times the volume. An
//     unknown value drops out of the distance, so V_PART wins.
//   - TALL (16.8 m high, 6 storeys of 2.8 m): V_ONE and V_SEVEN differ only in storeys. Storeys
//     are integers and must be normalised without integer division, so 6 lies
//     nearer 7 than 1.
//   - GABLE (10 m by 8 m, 6 m eave, 30 degree roof): V_EAVE and V_RIDGE differ only in
//     volume, eave × footprint against ridge × footprint. TABULA's V_C leaves out an
//     unheated attic, so the eave-based volume decides and V_EAVE wins.
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
	gableHouse(f, "GABLE", x+100, y, 200, 6, 4*math.Tan(math.Pi/6))
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	mustExec(t, ctx, `
		INSERT INTO city2tabula.tabula_variant (tabula_variant_code_id, tabula_variant_code, max_volume,
			footprint_area, number_of_storeys, area_total_roof, area_total_wall, area_total_floor)
		VALUES (1, 'V_FULL',  5000, 100, 4, 100, 400, 400),
		       (2, 'V_PART',  NULL, 100, 4, 100, 400, 400),
		       (3, 'V_ONE',   1680, 100, 1, 100, 672, 600),
		       (4, 'V_SEVEN', 1680, 100, 7, 100, 672, 600),
		       (5, 'V_EAVE',  480,   80, 2,  92.38, 234.48, 188.04),
		       (6, 'V_RIDGE', 664.75, 80, 2, 92.38, 234.48, 188.04)`)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	for id, want := range map[string]string{"BOX": "V_PART", "TALL": "V_SEVEN", "GABLE": "V_EAVE"} {
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

// TestPipeline_TabulaMatching_LabelIndependentOfOtherBuildings pins that a building's
// label depends only on its own values and the TABULA variants. BOX (10 m cube, 4
// storeys) differs from V_AREA only in storeys and from V_STOREY by a factor of two in
// volume and every area, so V_AREA is nearer on the variants' own scale. Neither the
// other buildings of the parts fixture nor the 100 m BIG extracted later may change that:
// scaled over all buildings, they stretch the volume and area ranges until the factor of
// two no longer counts and V_STOREY wins.
func TestPipeline_TabulaMatching_LabelIndependentOfOtherBuildings(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const x, y = -7000.0, 342800.0
	first := &partsFixture{nextID: 610000}
	first.solid(first.feature(901, "BOX"), box{x, y, x + 10, y + 10, 200, 210, "SENW", 200})
	mustExec(t, ctx, buildPartsFixture()+first.sql.String())
	seedDB(t)
	mustExec(t, ctx, `
		INSERT INTO city2tabula.tabula_variant (tabula_variant_code_id, tabula_variant_code, max_volume,
			footprint_area, number_of_storeys, area_total_roof, area_total_wall, area_total_floor)
		VALUES (1, 'V_AREA',   1000, 100, 1, 100, 400, 400),
		       (2, 'V_STOREY', 2000, 200, 4, 200, 800, 800)`)

	label := func() string {
		t.Helper()
		var got string
		if err := testPool.QueryRow(ctx,
			`SELECT COALESCE(tabula_variant_code, '') FROM city2tabula.lod2_building WHERE object_id = 'BOX'`,
		).Scan(&got); err != nil {
			t.Fatalf("read BOX label: %v", err)
		}
		return got
	}

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("first RunFeatureExtraction: %v", err)
	}
	if got := label(); got != "V_AREA" {
		t.Fatalf("BOX matched %q, want V_AREA", got)
	}

	second := &partsFixture{nextID: 620000}
	second.solid(second.feature(901, "BIG"), box{x + 500, y, x + 600, y + 100, 200, 210, "SENW", 200})
	mustExec(t, ctx, second.sql.String()+
		"UPDATE lod2.feature SET lineage = 'test-dataset' WHERE lineage IS NULL;")
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("second RunFeatureExtraction: %v", err)
	}
	if got := label(); got != "V_AREA" {
		t.Errorf("BOX matched %q after BIG was extracted, want V_AREA unchanged", got)
	}
}
