//go:build integration

package process_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// dumpAndRestoreCity2TabulaSchema replicates the export/import round trip used to share a
// corrected dataset between machines (heat-demand-models/export-data.sh): pg_dump the
// city2tabula schema, drop it, then pg_restore. Trigger DDL replayed during restore runs
// under the search_path pg_dump recorded for the dump (city2tabula only, not public),
// which is what silently dropped the footprint-geometry trigger before it was fixed to use
// schema-qualified public.ST_Equals instead of the search_path-dependent IS DISTINCT FROM.
func dumpAndRestoreCity2TabulaSchema(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	dumpPath := filepath.Join(t.TempDir(), "city2tabula.dump")

	dump := exec.Command("pg_dump", "-Fc", "--schema="+config.City2TabulaSchema, testConnStr, "-f", dumpPath)
	if out, err := dump.CombinedOutput(); err != nil {
		t.Fatalf("pg_dump failed: %v\noutput:\n%s", err, out)
	}

	if _, err := testPool.Exec(ctx, "DROP SCHEMA "+config.City2TabulaSchema+" CASCADE"); err != nil {
		t.Fatalf("failed to drop %s schema before restore: %v", config.City2TabulaSchema, err)
	}

	restore := exec.Command("pg_restore", "-d", testConnStr, dumpPath)
	out, err := restore.CombinedOutput()
	// pg_restore exits non-zero on any warning (including the ambiguous-operator error this
	// test guards against), so a restore this test relies on failing loudly is the point —
	// don't swallow it as a soft warning.
	if err != nil {
		t.Fatalf("pg_restore reported errors: %v\noutput:\n%s", err, out)
	}
}

// TestFootprintGeomTrigger_SurvivesDumpRestore guards the bug from work-tasks#19 /
// city2tabula#82: the footprint-geometry correction trigger must still fire correctly
// after the schema has been through a pg_dump --schema=city2tabula / pg_restore round
// trip, not just on a freshly created database (which is all
// TestFootprintGeomTrigger_RecomputesDerivedAttributes proves).
func TestFootprintGeomTrigger_SurvivesDumpRestore(t *testing.T) {
	_, buildingID := setupCorrectionAuditFixture(t)
	ctx := context.Background()

	before := readBuildingDims(t, ctx, buildingID)

	dumpAndRestoreCity2TabulaSchema(t)

	if _, err := testPool.Exec(ctx, `
		UPDATE city2tabula.lod2_building
		SET building_footprint_geom = ST_Multi(ST_Force3D(ST_MakeEnvelope(0, 0, 10, 10, 25832)))
		WHERE id = $1`, buildingID,
	); err != nil {
		t.Fatalf("failed to apply footprint correction after restore: %v", err)
	}

	var footprintArea float64
	var footprintComplexity int
	if err := testPool.QueryRow(ctx,
		`SELECT footprint_area, footprint_complexity FROM city2tabula.lod2_building WHERE id = $1`,
		buildingID,
	).Scan(&footprintArea, &footprintComplexity); err != nil {
		t.Fatalf("failed to read recomputed building after restore: %v", err)
	}

	if !almostEqual(footprintArea, 100.0) {
		t.Errorf("trigger did not fire after dump/restore: expected footprint_area = 100.00 (10x10 square), got %v (before update: %v) — trigger was likely dropped silently on restore",
			footprintArea, before.footprintArea)
	}
	if footprintComplexity != 1 {
		t.Errorf("trigger did not fire after dump/restore: expected footprint_complexity = 1, got %d", footprintComplexity)
	}
}
