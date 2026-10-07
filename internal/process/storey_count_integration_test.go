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

// TestPipeline_StoreyCount pins the storey rule of script 06 with a 2.8 m storey height:
//
//	GABLE     6 m eave, 2.31 m roof rise: 2 full storeys plus an attic storey
//	BUNGALOW  3 m eave, 1.5 m roof rise: 1 full storey, roof too low for an attic storey
//	FLAT      9 m flat box: 3 full storeys, no attic
//
// The attic floor area follows WoFlV § 4 over each roof face's linear height profile
// above the eave, and counts towards area_total_floor only with an attic storey.
func TestPipeline_StoreyCount(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const y, z0 = 342800.0, 200.0
	gableRise := 4 * math.Tan(math.Pi/6)
	f := &partsFixture{nextID: 700000}
	gableHouse(f, "GABLE", -6000, y, z0, 6, gableRise)
	gableHouse(f, "BUNGALOW", -5970, y, z0, 3, 1.5)
	f.solid(f.feature(901, "FLAT"), box{-5940, y, -5930, y + 10, z0, z0 + 9, "SENW", z0})
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	// WoFlV share of a roof face's plan area: full above 2 m, half from 1 to 2 m.
	woflv := func(rise float64) float64 {
		above := func(h float64) float64 { return math.Max(0, math.Min(1, (rise-h)/rise)) }
		return above(2) + 0.5*(above(1)-above(2))
	}
	type want struct {
		full, storeys int
		attic         bool
		atticArea     float64
		floorArea     float64
	}
	gableAttic := 80 * woflv(gableRise)
	for id, w := range map[string]want{
		"GABLE":    {2, 3, true, gableAttic, 2*80 + gableAttic},
		"BUNGALOW": {1, 1, false, 80 * woflv(1.5), 80},
		"FLAT":     {3, 3, false, 0, 300},
	} {
		var g want
		if err := testPool.QueryRow(ctx, `
			SELECT full_storeys, number_of_storeys, attic_storey, attic_floor_area, area_total_floor
			FROM city2tabula.lod2_building WHERE object_id = $1`, id,
		).Scan(&g.full, &g.storeys, &g.attic, &g.atticArea, &g.floorArea); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if g.full != w.full || g.storeys != w.storeys || g.attic != w.attic ||
			math.Abs(g.atticArea-w.atticArea) > 0.05 || math.Abs(g.floorArea-w.floorArea) > 0.05 {
			t.Errorf("%s: got %+v, want %+v", id, g, w)
		}
	}
}
