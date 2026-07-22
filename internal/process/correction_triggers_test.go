//go:build integration

package process_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// setupCorrectionAuditFixture resets the schemas, seeds Germany's LOD2 fixture, and runs
// feature extraction once — the shared starting point for every test in this file. Returns
// the pipeline config (needed to re-run extraction) and one building's id to correct against.
func setupCorrectionAuditFixture(t *testing.T) (cfg *config.Config, buildingID string) {
	t.Helper()
	ctx := context.Background()

	tc := pipelineTestCase{
		country: "germany",
		srid:    "25832",
		seeds: []string{
			"testdata/germany/seed_lod2.sql",
			"testdata/germany/seed_tabula_variant.sql",
		},
	}
	for _, seed := range tc.seeds {
		if _, err := os.Stat(seed); os.IsNotExist(err) {
			t.Skipf("seed file not found, skipping: %s", seed)
		}
	}

	resetSchemas(t)
	cfg = pipelineConfig(tc)

	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	seedDB(t, tc.seeds...)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	if err := testPool.QueryRow(ctx,
		`SELECT id FROM city2tabula.lod2_building LIMIT 1`,
	).Scan(&buildingID); err != nil {
		t.Fatalf("failed to pick a building row: %v", err)
	}

	return cfg, buildingID
}

func timestampsOf(t *testing.T, ctx context.Context, buildingID string) (createdAt, updatedAt time.Time) {
	t.Helper()
	if err := testPool.QueryRow(ctx,
		`SELECT created_at, updated_at FROM city2tabula.lod2_building WHERE id = $1`, buildingID,
	).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatalf("failed to read timestamps for %s: %v", buildingID, err)
	}
	return createdAt, updatedAt
}

func TestCorrectionAuditTrigger_Baseline(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	createdAt, updatedAt := timestampsOf(t, ctx, buildingID)
	if !createdAt.Equal(updatedAt) {
		t.Errorf("expected created_at == updated_at right after extraction, got created_at=%v updated_at=%v", createdAt, updatedAt)
	}
}

func TestCorrectionAuditTrigger_RealCorrectionBumpsUpdatedAt(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	createdAt, _ := timestampsOf(t, ctx, buildingID)

	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET room_height = room_height + 0.1 WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply correction: %v", err)
	}

	_, updatedAt := timestampsOf(t, ctx, buildingID)
	if !updatedAt.After(createdAt) {
		t.Errorf("expected updated_at > created_at after a real correction, got created_at=%v updated_at=%v", createdAt, updatedAt)
	}
}

func TestCorrectionAuditTrigger_NoOpUpdateDoesNotBumpUpdatedAt(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	// A real correction first, so updated_at has already moved once.
	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET room_height = room_height + 0.1 WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply correction: %v", err)
	}
	_, before := timestampsOf(t, ctx, buildingID)

	// Same value written back — should be a no-op under the WHEN guard.
	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET room_height = room_height WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply no-op update: %v", err)
	}
	_, after := timestampsOf(t, ctx, buildingID)

	if !after.Equal(before) {
		t.Errorf("expected a no-op UPDATE to leave updated_at unchanged, got before=%v after=%v", before, after)
	}
}

func TestCorrectionAuditTrigger_RerunDoesNotCorruptTimestamps(t *testing.T) {
	cfg, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	if _, err := testPool.Exec(ctx,
		`UPDATE city2tabula.lod2_building SET room_height = room_height + 0.1 WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply correction: %v", err)
	}
	_, corrected := timestampsOf(t, ctx, buildingID)

	var countBefore int
	if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM city2tabula.lod2_building`).Scan(&countBefore); err != nil {
		t.Fatalf("failed to count buildings: %v", err)
	}

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("second RunFeatureExtraction: %v", err)
	}

	var countAfter int
	if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM city2tabula.lod2_building`).Scan(&countAfter); err != nil {
		t.Fatalf("failed to count buildings after re-run: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("expected building count unchanged after a re-run, got before=%d after=%d", countBefore, countAfter)
	}

	_, afterRerun := timestampsOf(t, ctx, buildingID)
	if !afterRerun.Equal(corrected) {
		t.Errorf("expected updated_at to survive a re-run untouched, got before=%v after=%v", corrected, afterRerun)
	}
}

func TestCorrectionAuditTrigger_FunctionIsSharedAcrossSchemas(t *testing.T) {
	setupCorrectionAuditFixture(t)
	ctx := context.Background()

	var exists bool
	if err := testPool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM pg_proc
			WHERE proname = 'touch_updated_at' AND pronamespace = 'city2tabula'::regnamespace
		)`,
	).Scan(&exists); err != nil {
		t.Fatalf("failed to check for touch_updated_at function: %v", err)
	}
	if !exists {
		t.Error("expected city2tabula.touch_updated_at() to exist as a single, schema-generic function")
	}
}

func TestCorrectionAuditTrigger_AllFiveEnabledTogether(t *testing.T) {
	setupCorrectionAuditFixture(t)
	ctx := context.Background()

	var enabledCount int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM pg_trigger t
		 JOIN pg_class c ON c.oid = t.tgrelid
		 JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'city2tabula' AND c.relname = 'lod2_building'
		   AND NOT t.tgisinternal AND t.tgenabled <> 'D'`,
	).Scan(&enabledCount); err != nil {
		t.Fatalf("failed to count enabled triggers: %v", err)
	}
	if enabledCount != 5 {
		t.Errorf("expected all 5 correction triggers enabled after extraction, got %d", enabledCount)
	}
}
