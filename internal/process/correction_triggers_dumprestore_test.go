//go:build integration

package process_test

import (
	"bytes"
	"context"
	"testing"

	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// pgEnv authenticates pg_dump/pg_restore inside the container the same way the test DB
// itself was configured in TestMain (POSTGRES_USER=test, POSTGRES_PASSWORD=test).
var pgEnv = []string{"PGPASSWORD=test"}

// execInContainer runs a command inside the running PostGIS container (rather than on the
// host) so pg_dump/pg_restore always match the server's own version — a host-installed
// client can be older or newer than the container's Postgres and refuse to run, which is
// exactly what happened the first time this test ran in CI (client 16.14 vs. server 17.0).
func execInContainer(t *testing.T, cmd []string) string {
	t.Helper()
	ctx := context.Background()

	exitCode, reader, err := testContainer.Exec(ctx, cmd, tcexec.WithEnv(pgEnv))
	if err != nil {
		t.Fatalf("failed to exec %v in container: %v", cmd, err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("failed to read output of %v: %v", cmd, err)
	}
	output := buf.String()

	if exitCode != 0 {
		t.Fatalf("%v exited %d:\n%s", cmd, exitCode, output)
	}
	return output
}

// dumpAndRestoreCity2TabulaSchema replicates the export/import round trip used to share a
// corrected dataset between machines (heat-demand-models/export-data.sh): pg_dump the
// city2tabula schema, drop it, then pg_restore. Trigger DDL replayed during restore runs
// under the search_path pg_dump recorded for the dump (city2tabula only, not public),
// which is what silently dropped the footprint-geometry trigger before it was fixed to use
// schema-qualified public.ST_Equals instead of the search_path-dependent IS DISTINCT FROM.
func dumpAndRestoreCity2TabulaSchema(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	const dumpPath = "/tmp/city2tabula.dump"

	execInContainer(t, []string{
		"pg_dump", "-h", "localhost", "-U", "test", "-d", "city2tabula_test",
		"-Fc", "--schema=" + config.City2TabulaSchema, "-f", dumpPath,
	})

	if _, err := testPool.Exec(ctx, "DROP SCHEMA "+config.City2TabulaSchema+" CASCADE"); err != nil {
		t.Fatalf("failed to drop %s schema before restore: %v", config.City2TabulaSchema, err)
	}

	// pg_restore exits non-zero on any error (including the ambiguous-operator error this
	// test guards against), so a restore this test relies on failing loudly is the point —
	// execInContainer treats that as a hard failure rather than a swallowed warning.
	execInContainer(t, []string{
		"pg_restore", "-h", "localhost", "-U", "test", "-d", "city2tabula_test", dumpPath,
	})
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
