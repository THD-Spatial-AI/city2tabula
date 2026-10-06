//go:build integration

package process_test

import (
	"context"
	"math"
	"testing"
)

// floatTolerance absorbs float64 round-trip noise through Postgres double precision
// and the triggers' own ROUND(...::numeric, 2) calls, not real disagreement.
const floatTolerance = 0.01

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTolerance
}

// round2 mirrors the ROUND(x::numeric, 2) the trigger functions apply server-side,
// so expected values computed here compare cleanly against what got persisted.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// buildingDims is the subset of _building columns the correction triggers read or
// write, used both to capture "before" state and to compute expected "after" values.
type buildingDims struct {
	minHeight           float64
	maxHeight           float64
	maxVolume           float64
	footprintArea       float64
	footprintComplexity int
	roofComplexity      int
	areaTotalRoof       float64
	areaTotalWall       float64
	areaTotalFloor      float64
	numberOfStoreys     int
	fullStoreys         int
	atticStorey         bool
	atticFloorArea      float64
}

// atticArea is the attic floor area that counts towards area_total_floor (script 06).
func (d buildingDims) atticArea() float64 {
	if d.atticStorey {
		return d.atticFloorArea
	}
	return 0
}

func readBuildingDims(t *testing.T, ctx context.Context, buildingID string) buildingDims {
	t.Helper()
	var d buildingDims
	if err := testPool.QueryRow(ctx, `
		SELECT min_height, max_height, max_volume, footprint_area, footprint_complexity,
		       roof_complexity, area_total_roof, area_total_wall, area_total_floor,
		       number_of_storeys, full_storeys, attic_storey, COALESCE(attic_floor_area, 0)
		FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&d.minHeight, &d.maxHeight, &d.maxVolume, &d.footprintArea, &d.footprintComplexity,
		&d.roofComplexity, &d.areaTotalRoof, &d.areaTotalWall, &d.areaTotalFloor,
		&d.numberOfStoreys, &d.fullStoreys, &d.atticStorey, &d.atticFloorArea); err != nil {
		t.Fatalf("failed to read building dims for %s: %v", buildingID, err)
	}
	return d
}

// TestFootprintGeomTrigger_RecomputesDerivedAttributes drives trg_footprint_geom_change
// with a 10x10m square at the origin: its area, boundary vertex count, and centroid are
// all hand-computable, so the recompute can be checked against known values instead of
// just "did footprint_area change".
func TestFootprintGeomTrigger_RecomputesDerivedAttributes(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)

	if _, err := testPool.Exec(ctx, `
		UPDATE city2tabula.lod2_building
		SET building_footprint_geom = ST_Multi(ST_Force3D(ST_MakeEnvelope(0, 0, 10, 10, 25832)))
		WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply footprint correction: %v", err)
	}

	var footprintArea, centroidX, centroidY, minVolume, maxVolume, areaTotalFloor float64
	var footprintComplexity int
	if err := testPool.QueryRow(ctx, `
		SELECT footprint_area, footprint_complexity,
		       ST_X(building_centroid_geom), ST_Y(building_centroid_geom),
		       min_volume, max_volume, area_total_floor
		FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&footprintArea, &footprintComplexity, &centroidX, &centroidY,
		&minVolume, &maxVolume, &areaTotalFloor); err != nil {
		t.Fatalf("failed to read recomputed building: %v", err)
	}

	if !almostEqual(footprintArea, 100.0) {
		t.Errorf("expected footprint_area = 100.00 (10x10 square), got %v", footprintArea)
	}
	if footprintComplexity != 1 {
		t.Errorf("expected footprint_complexity = 1 (5-vertex envelope boundary), got %d", footprintComplexity)
	}
	if !almostEqual(centroidX, 5.0) || !almostEqual(centroidY, 5.0) {
		t.Errorf("expected building_centroid_geom = (5, 5), got (%v, %v)", centroidX, centroidY)
	}

	wantMinVolume := round2(before.minHeight * 100.0)
	if !almostEqual(minVolume, wantMinVolume) {
		t.Errorf("expected min_volume = min_height(%v) * 100 = %v, got %v", before.minHeight, wantMinVolume, minVolume)
	}
	wantMaxVolume := round2(before.maxHeight * 100.0)
	if !almostEqual(maxVolume, wantMaxVolume) {
		t.Errorf("expected max_volume = max_height(%v) * 100 = %v, got %v", before.maxHeight, wantMaxVolume, maxVolume)
	}
	wantAreaTotalFloor := round2(100.0*float64(before.fullStoreys) + before.atticArea())
	if !almostEqual(areaTotalFloor, wantAreaTotalFloor) {
		t.Errorf("expected area_total_floor = 100 * full_storeys(%d) + attic area = %v, got %v",
			before.fullStoreys, wantAreaTotalFloor, areaTotalFloor)
	}
}

// TestVariantDimsTrigger_RematchesToNearestVariant drives trg_variant_dims_change by
// inserting a synthetic tabula_variant that exactly matches the building on every
// dimension the matching formula uses (copying the building's own current values for
// max_volume/footprint_complexity/roof_complexity/area_total_roof/area_total_wall,
// then setting the building's footprint_area/full_storeys/area_total_floor to the
// same distinctive numbers used on the synthetic row). Distance to that variant is then
// exactly 0 — the unambiguous nearest match — so the rematch can be checked against a
// known tabula_variant_code_id instead of just "did it change".
func TestVariantDimsTrigger_RematchesToNearestVariant(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)

	const (
		syntheticCodeID        = 900001
		syntheticFootprintArea = 55555.55
		syntheticStoreys       = 77
		syntheticFloorArea     = 66666.66
	)

	if _, err := testPool.Exec(ctx, `
		INSERT INTO city2tabula.tabula_variant (
			tabula_variant_code_id, tabula_variant_code, max_volume, footprint_area,
			number_of_storeys, footprint_complexity, roof_complexity,
			area_total_roof, area_total_wall, area_total_floor
		) VALUES ($1, 'TEST.SYNTHETIC.EXACT.MATCH', $2, $3, $4, $5, $6, $7, $8, $9)`,
		syntheticCodeID, before.maxVolume, syntheticFootprintArea, syntheticStoreys,
		before.footprintComplexity, before.roofComplexity, before.areaTotalRoof, before.areaTotalWall,
		syntheticFloorArea,
	); err != nil {
		t.Fatalf("failed to insert synthetic variant: %v", err)
	}

	if _, err := testPool.Exec(ctx, `
		UPDATE city2tabula.lod2_building
		SET footprint_area = $1, full_storeys = $2, area_total_floor = $3
		WHERE id = $4`,
		syntheticFootprintArea, syntheticStoreys, syntheticFloorArea, buildingID,
	); err != nil {
		t.Fatalf("failed to apply correction: %v", err)
	}

	var gotCodeID int
	if err := testPool.QueryRow(ctx,
		`SELECT tabula_variant_code_id FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&gotCodeID); err != nil {
		t.Fatalf("failed to read rematched variant: %v", err)
	}
	if gotCodeID != syntheticCodeID {
		t.Errorf("expected rematch to the exact-match synthetic variant %d, got %d", syntheticCodeID, gotCodeID)
	}
}

// TestStoreyHeightTrigger_CascadesToStoreysAndFloorArea drives trg_storey_height_change
// by setting storey_height = min_height / 3, so full_storeys lands on 3. It also checks
// the cascade into trg_storeys_change: area_total_floor recomputes from the new count,
// and storey_height settles back on min_height / 3 (the trigger chain's self-check, not
// an infinite loop; see 03_create_correction_triggers.sql).
func TestStoreyHeightTrigger_CascadesToStoreysAndFloorArea(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)
	if before.minHeight <= 0 {
		t.Fatalf("fixture building has non-positive min_height (%v); can't drive this trigger", before.minHeight)
	}

	const wantFull = 3
	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET storey_height = $1 WHERE id = $2`,
		before.minHeight/wantFull, buildingID,
	); err != nil {
		t.Fatalf("failed to apply storey_height correction: %v", err)
	}

	after := readBuildingDims(t, ctx, buildingID)
	var gotStoreyHeight float64
	if err := testPool.QueryRow(ctx,
		`SELECT storey_height FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&gotStoreyHeight); err != nil {
		t.Fatalf("failed to read storey_height: %v", err)
	}

	wantStoreys := wantFull
	if before.atticStorey {
		wantStoreys++
	}
	if after.fullStoreys != wantFull || after.numberOfStoreys != wantStoreys {
		t.Errorf("expected full_storeys %d and number_of_storeys %d, got %d and %d",
			wantFull, wantStoreys, after.fullStoreys, after.numberOfStoreys)
	}
	if want := round2(before.minHeight / wantFull); !almostEqual(gotStoreyHeight, want) {
		t.Errorf("expected storey_height settled at min_height/full_storeys = %v, got %v", want, gotStoreyHeight)
	}
	if want := round2(before.footprintArea*wantFull + before.atticArea()); !almostEqual(after.areaTotalFloor, want) {
		t.Errorf("expected area_total_floor = footprint_area * full_storeys + attic area = %v, got %v", want, after.areaTotalFloor)
	}
}

// TestStoreysTrigger_CascadesToStoreyHeightAndFloorArea drives trg_storeys_change by
// editing number_of_storeys directly (the mirror image of the storey_height edit
// above). The attic storey is geometric, so the edit lands on full_storeys, and
// min_height = storey_height * full_storeys holds from this side too.
func TestStoreysTrigger_CascadesToStoreyHeightAndFloorArea(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)
	if before.minHeight <= 0 {
		t.Fatalf("fixture building has non-positive min_height (%v); can't drive this trigger", before.minHeight)
	}

	wantStoreys := before.numberOfStoreys + 1 // guaranteed different from baseline
	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET number_of_storeys = $1 WHERE id = $2`,
		wantStoreys, buildingID,
	); err != nil {
		t.Fatalf("failed to apply number_of_storeys correction: %v", err)
	}

	after := readBuildingDims(t, ctx, buildingID)
	var gotStoreyHeight float64
	if err := testPool.QueryRow(ctx,
		`SELECT storey_height FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&gotStoreyHeight); err != nil {
		t.Fatalf("failed to read storey_height: %v", err)
	}

	wantFull := wantStoreys
	if before.atticStorey {
		wantFull--
	}
	if after.numberOfStoreys != wantStoreys || after.fullStoreys != wantFull {
		t.Errorf("expected number_of_storeys %d and full_storeys %d to settle, got %d and %d",
			wantStoreys, wantFull, after.numberOfStoreys, after.fullStoreys)
	}
	if want := round2(before.minHeight / float64(wantFull)); !almostEqual(gotStoreyHeight, want) {
		t.Errorf("expected storey_height = min_height/full_storeys = %v, got %v", want, gotStoreyHeight)
	}
	if want := round2(before.footprintArea*float64(wantFull) + before.atticArea()); !almostEqual(after.areaTotalFloor, want) {
		t.Errorf("expected area_total_floor = footprint_area * full_storeys + attic area = %v, got %v", want, after.areaTotalFloor)
	}
}
