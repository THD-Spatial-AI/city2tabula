//go:build integration

package process_test

import (
	"context"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// eligibleBuildingFeatureIDs returns the building_feature_ids GetGridBatches should
// be able to place into a cell: those with both a footprint geometry and an
// object_id set, matching GetGridBatches's own WHERE clause.
func eligibleBuildingFeatureIDs(t *testing.T, ctx context.Context) map[int64]bool {
	t.Helper()
	rows, err := testPool.Query(ctx, `
		SELECT building_feature_id FROM city2tabula.lod2_building
		WHERE building_footprint_geom IS NOT NULL AND object_id IS NOT NULL`,
	)
	if err != nil {
		t.Fatalf("failed to query eligible buildings: %v", err)
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("failed to scan building_feature_id: %v", err)
		}
		ids[id] = true
	}
	return ids
}

// batchTotal sums batch sizes; batchUnion flattens all batches into a set, so tests
// can tell "every eligible building appears somewhere" apart from "no duplicates
// across cells" — a building whose footprint straddles a grid cell boundary is
// expected to appear in more than one cell's batch (see GetGridBatches's own
// ST_Intersects join), so union size, not total size, is the coverage signal.
func batchUnion(batches [][]int64) map[int64]bool {
	union := make(map[int64]bool)
	for _, b := range batches {
		for _, id := range b {
			union[id] = true
		}
	}
	return union
}

func batchTotal(batches [][]int64) int {
	total := 0
	for _, b := range batches {
		total += len(b)
	}
	return total
}

// TestGetGridBatches_SingleLargeCellCoversAllEligibleBuildings uses a grid cell far
// larger than the fixture's extent, so every eligible building falls into exactly
// one cell — checkable against a known set (queried independently) instead of just
// "some batches came back".
func TestGetGridBatches_SingleLargeCellCoversAllEligibleBuildings(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	want := eligibleBuildingFeatureIDs(t, ctx)
	if len(want) == 0 {
		t.Fatal("fixture produced no eligible buildings (footprint + object_id); can't test batching")
	}

	const hugeGridSizeM = 100_000 // 100km: comfortably covers the whole fixture extent
	batches, err := process.GetGridBatches(testPool, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2, hugeGridSizeM, 0)
	if err != nil {
		t.Fatalf("GetGridBatches: %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("expected exactly 1 grid cell to cover the whole fixture, got %d", len(batches))
	}
	got := batchUnion(batches)
	if len(got) != len(want) {
		t.Errorf("expected %d buildings in the single batch, got %d", len(want), len(got))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("expected building_feature_id %d in the batch, was missing", id)
		}
	}
}

// TestGetGridBatches_BuildingLimitCapsTotal drives the buildingLimit parameter and
// checks the total across all batches never exceeds it.
func TestGetGridBatches_BuildingLimitCapsTotal(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)

	const limit = 10
	batches, err := process.GetGridBatches(testPool, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2, 100_000, limit)
	if err != nil {
		t.Fatalf("GetGridBatches: %v", err)
	}

	total := batchTotal(batches)
	if total == 0 {
		t.Fatal("expected at least 1 building under the limit, got 0")
	}
	if total > limit {
		t.Errorf("expected buildingLimit=%d to cap the total across batches, got %d", limit, total)
	}
}

// TestGetGridBatches_SmallerGridProducesMoreCells is a coarse but real correctness
// check that gridSizeM actually drives the cell count, not a hardcoded single cell:
// a much smaller grid over the same fixture must split it into more (non-empty)
// cells than one huge cell does. Coverage must hold regardless of cell size: every
// eligible building still appears in at least one cell, even split across many.
func TestGetGridBatches_SmallerGridProducesMoreCells(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	bigCellBatches, err := process.GetGridBatches(testPool, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2, 100_000, 0)
	if err != nil {
		t.Fatalf("GetGridBatches (large grid): %v", err)
	}

	smallCellBatches, err := process.GetGridBatches(testPool, cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2, 50, 0)
	if err != nil {
		t.Fatalf("GetGridBatches (small grid): %v", err)
	}

	if len(smallCellBatches) <= len(bigCellBatches) {
		t.Errorf("expected a 50m grid to produce more cells than a 100km grid over the same fixture, got %d vs %d",
			len(smallCellBatches), len(bigCellBatches))
	}

	want := eligibleBuildingFeatureIDs(t, ctx)
	got := batchUnion(smallCellBatches)
	if len(got) != len(want) {
		t.Errorf("expected the small-grid union to cover all %d eligible buildings, got %d", len(want), len(got))
	}
}
