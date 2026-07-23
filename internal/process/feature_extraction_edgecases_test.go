//go:build integration

package process_test

import (
	"context"
	"os"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// setupGermanyLOD2 resets the schemas, seeds Germany's LOD2 fixture, and runs the
// DB setup + import steps that come before RunFeatureExtraction — without running
// extraction itself, so each test here can adjust cfg or the seeded data first.
func setupGermanyLOD2(t *testing.T) *pipelineTestCase {
	t.Helper()

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
	if err := db.RunCity2TabulaDBSetup(pipelineConfig(tc), testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	seedDB(t, tc.seeds...)

	return &tc
}

// TestRunFeatureExtraction_BuildingLimitTruncatesCount drives the BUILDING_LIMIT
// branch in RunFeatureExtraction (feature_extraction.go): "ids = ids[:limit]" when
// fewer buildings are requested than the fixture provides. Germany's LOD2 fixture
// has more than 5 buildings, so a limit of 5 only passes if it actually clipped.
func TestRunFeatureExtraction_BuildingLimitTruncatesCount(t *testing.T) {
	tc := setupGermanyLOD2(t)
	ctx := context.Background()

	cfg := pipelineConfig(*tc)
	const limit = 5
	cfg.Batch.BuildingLimit = limit

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var count int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM city2tabula.lod2_building`,
	).Scan(&count); err != nil {
		t.Fatalf("failed to count buildings: %v", err)
	}
	if count != limit {
		t.Errorf("expected BUILDING_LIMIT=%d to cap processed buildings at %d, got %d", limit, limit, count)
	}
}

// TestRunFeatureExtraction_NoBuildingsIsANoOp drives the "no buildings found in any
// configured LOD schema" early return in RunFeatureExtraction: with the CityDB
// feature table emptied of building-class rows (but the lod2 schema itself intact,
// unlike an unseeded DB which would fail on a missing-relation error instead), the
// run must exit cleanly with no error and no rows written, not panic or fail.
func TestRunFeatureExtraction_NoBuildingsIsANoOp(t *testing.T) {
	tc := setupGermanyLOD2(t)
	ctx := context.Background()

	if _, err := testPool.Exec(ctx, `TRUNCATE lod2.feature CASCADE`); err != nil {
		t.Fatalf("failed to empty lod2.feature: %v", err)
	}

	cfg := pipelineConfig(*tc)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("expected a no-op (nil error) when CityDB has no buildings, got: %v", err)
	}

	var count int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM city2tabula.lod2_building`,
	).Scan(&count); err != nil {
		t.Fatalf("failed to count buildings: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no buildings written when CityDB has none, got %d", count)
	}
}

// TestEnableCorrectionTriggers_PartialFailureRollsBackAll verifies the transaction
// wrap added around the five ALTER TABLE ... ENABLE TRIGGER statements in
// EnableCorrectionTriggers (feature_extraction.go): dropping one trigger partway
// through the watched list forces the 3rd of 5 statements to fail, and the whole
// batch — including the first two, which would have succeeded standalone — must
// roll back to "still disabled" rather than leaving a partial mix.
func TestEnableCorrectionTriggers_PartialFailureRollsBackAll(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t) // leaves all 5 real triggers enabled
	ctx := context.Background()

	const table = "city2tabula.lod2_building"
	triggers := []string{
		"lod2_trg_footprint_geom_change",
		"lod2_trg_variant_dims_change",
		"lod2_trg_room_height_change",
		"lod2_trg_storeys_change",
		"lod2_trg_touch_updated_at",
	}
	for _, trg := range triggers {
		if _, err := testPool.Exec(ctx,
			`ALTER TABLE `+table+` DISABLE TRIGGER `+trg,
		); err != nil {
			t.Fatalf("failed to disable %s ahead of the test: %v", trg, err)
		}
	}
	// Drop the 3rd of 5 triggers so EnableCorrectionTriggers's loop fails partway
	// through: the first two ENABLE statements succeed inside the transaction
	// before the failure, proving the rollback reverts already-applied work too.
	if _, err := testPool.Exec(ctx,
		`DROP TRIGGER lod2_trg_room_height_change ON `+table,
	); err != nil {
		t.Fatalf("failed to drop lod2_trg_room_height_change: %v", err)
	}

	err := process.EnableCorrectionTriggers(testPool, cfg, "lod2")
	if err == nil {
		t.Fatal("expected EnableCorrectionTriggers to fail after a trigger was dropped, got nil error")
	}

	var stillEnabled int
	if err := testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM pg_trigger t
		 JOIN pg_class c ON c.oid = t.tgrelid
		 JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'city2tabula' AND c.relname = 'lod2_building'
		   AND NOT t.tgisinternal AND t.tgenabled <> 'D'`,
	).Scan(&stillEnabled); err != nil {
		t.Fatalf("failed to count enabled triggers: %v", err)
	}
	if stillEnabled != 0 {
		t.Errorf("expected all triggers to remain disabled after a rolled-back partial failure, got %d enabled", stillEnabled)
	}
}
