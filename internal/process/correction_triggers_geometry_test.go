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
}

func readBuildingDims(t *testing.T, ctx context.Context, buildingID string) buildingDims {
	t.Helper()
	var d buildingDims
	if err := testPool.QueryRow(ctx, `
		SELECT min_height, max_height, max_volume, footprint_area, footprint_complexity,
		       roof_complexity, area_total_roof, area_total_wall, area_total_floor,
		       number_of_storeys
		FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&d.minHeight, &d.maxHeight, &d.maxVolume, &d.footprintArea, &d.footprintComplexity,
		&d.roofComplexity, &d.areaTotalRoof, &d.areaTotalWall, &d.areaTotalFloor,
		&d.numberOfStoreys); err != nil {
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
	wantAreaTotalFloor := round2(100.0 * float64(before.numberOfStoreys))
	if !almostEqual(areaTotalFloor, wantAreaTotalFloor) {
		t.Errorf("expected area_total_floor = 100 * number_of_storeys(%d) = %v, got %v",
			before.numberOfStoreys, wantAreaTotalFloor, areaTotalFloor)
	}
}

// TestVariantDimsTrigger_RematchesToNearestVariant drives trg_variant_dims_change by
// inserting a synthetic tabula_variant that exactly matches the building on every
// dimension the matching formula uses (copying the building's own current values for
// max_volume/footprint_complexity/roof_complexity/area_total_roof/area_total_wall,
// then setting the building's footprint_area/number_of_storeys/area_total_floor to the
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
		SET footprint_area = $1, number_of_storeys = $2, area_total_floor = $3
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

// TestRoomHeightTrigger_CascadesToStoreysAndFloorArea drives trg_room_height_change by
// setting room_height = min_height / 3, so the recomputed number_of_storeys lands
// exactly on 3 (mod float noise) — checkable against a known value. It also verifies
// the cascade into trg_storeys_change: area_total_floor recomputes from the new storey
// count, and room_height settles back close to the value just set (the trigger chain's
// self-check, not an infinite loop — see 03_create_correction_triggers.sql).
func TestRoomHeightTrigger_CascadesToStoreysAndFloorArea(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)
	if before.minHeight <= 0 {
		t.Fatalf("fixture building has non-positive min_height (%v); can't drive this trigger", before.minHeight)
	}

	const wantStoreys = 3
	newRoomHeight := before.minHeight / wantStoreys

	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET room_height = $1 WHERE id = $2`,
		newRoomHeight, buildingID,
	); err != nil {
		t.Fatalf("failed to apply room_height correction: %v", err)
	}

	var gotStoreys int
	var gotRoomHeight, gotAreaTotalFloor float64
	if err := testPool.QueryRow(ctx,
		`SELECT number_of_storeys, room_height, area_total_floor FROM city2tabula.lod2_building WHERE id = $1`,
		buildingID,
	).Scan(&gotStoreys, &gotRoomHeight, &gotAreaTotalFloor); err != nil {
		t.Fatalf("failed to read cascaded building: %v", err)
	}

	if gotStoreys != wantStoreys {
		t.Errorf("expected number_of_storeys = round(min_height/room_height) = %d, got %d", wantStoreys, gotStoreys)
	}
	wantRoomHeight := round2(before.minHeight / float64(wantStoreys))
	if !almostEqual(gotRoomHeight, wantRoomHeight) {
		t.Errorf("expected room_height settled back to min_height/number_of_storeys = %v, got %v", wantRoomHeight, gotRoomHeight)
	}
	wantAreaTotalFloor := round2(before.footprintArea * float64(wantStoreys))
	if !almostEqual(gotAreaTotalFloor, wantAreaTotalFloor) {
		t.Errorf("expected area_total_floor = footprint_area * number_of_storeys = %v, got %v", wantAreaTotalFloor, gotAreaTotalFloor)
	}
}

// TestStoreysTrigger_CascadesToRoomHeightAndFloorArea drives trg_storeys_change by
// editing number_of_storeys directly (the mirror image of the room_height-edit case
// above), and verifies min_height = room_height * number_of_storeys holds from this
// side too: room_height and area_total_floor both recompute, and number_of_storeys
// itself survives the room_height-trigger's own round-trip unchanged.
func TestStoreysTrigger_CascadesToRoomHeightAndFloorArea(t *testing.T) {
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

	var gotStoreys int
	var gotRoomHeight, gotAreaTotalFloor float64
	if err := testPool.QueryRow(ctx,
		`SELECT number_of_storeys, room_height, area_total_floor FROM city2tabula.lod2_building WHERE id = $1`,
		buildingID,
	).Scan(&gotStoreys, &gotRoomHeight, &gotAreaTotalFloor); err != nil {
		t.Fatalf("failed to read cascaded building: %v", err)
	}

	if gotStoreys != wantStoreys {
		t.Errorf("expected number_of_storeys to settle at the directly-set value %d, got %d (room_height mirror may have overwritten it)",
			wantStoreys, gotStoreys)
	}
	wantRoomHeight := round2(before.minHeight / float64(wantStoreys))
	if !almostEqual(gotRoomHeight, wantRoomHeight) {
		t.Errorf("expected room_height = min_height/number_of_storeys = %v, got %v", wantRoomHeight, gotRoomHeight)
	}
	wantAreaTotalFloor := round2(before.footprintArea * float64(wantStoreys))
	if !almostEqual(gotAreaTotalFloor, wantAreaTotalFloor) {
		t.Errorf("expected area_total_floor = footprint_area * number_of_storeys = %v, got %v", wantAreaTotalFloor, gotAreaTotalFloor)
	}
}
