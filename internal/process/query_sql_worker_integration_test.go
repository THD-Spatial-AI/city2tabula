//go:build integration

package process_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestRunFeatureExtraction_BadLod2SchemaPropagatesQueryError drives
// getBuildingObjectClassIDs's real query-error branch (query.go) through
// RunFeatureExtraction's own error-wrap: a schema name that doesn't exist at
// all produces a genuine Postgres "relation does not exist" error, not a
// simulated one.
func TestRunFeatureExtraction_BadLod2SchemaPropagatesQueryError(t *testing.T) {
	tc := setupGermanyLOD2(t)

	cfg := pipelineConfig(*tc)
	cfg.DB.Schemas.Lod2 = "schema_that_does_not_exist"

	err := process.RunFeatureExtraction(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error for a nonexistent LOD2 schema, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get LOD2 building IDs") {
		t.Errorf("expected RunFeatureExtraction's own error wrap, got: %v", err)
	}
}

// TestRunFeatureExtraction_MissingCity2TabulaSchemaPropagatesProcessedIDsError
// drives getProcessedBuildingFeatureIDs's real query-error branch: real LOD2
// CityDB data exists (so getBuildingIDsFromCityDB succeeds), but the
// city2tabula schema hasn't been set up yet, so the already-processed check
// fails for real.
func TestRunFeatureExtraction_MissingCity2TabulaSchemaPropagatesProcessedIDsError(t *testing.T) {
	tc := setupGermanyLOD2(t)
	ctx := context.Background()

	// setupGermanyLOD2 already ran db.RunCity2TabulaDBSetup, so drop it back off
	// to reproduce "CityDB data exists, city2tabula schema does not".
	if _, err := testPool.Exec(ctx, `DROP SCHEMA IF EXISTS city2tabula CASCADE`); err != nil {
		t.Fatalf("failed to drop city2tabula schema: %v", err)
	}

	cfg := pipelineConfig(*tc)
	err := process.RunFeatureExtraction(cfg, testPool)
	if err == nil {
		t.Fatal("expected an error when city2tabula schema is missing, got nil")
	}
	if !strings.Contains(err.Error(), "failed to filter already-processed LOD2 building IDs") {
		t.Errorf("expected RunFeatureExtraction's own error wrap, got: %v", err)
	}
}

// TestRunTaskWithRetry_ExecuteSQLScriptFailureIsWrapped drives runSingleTask's
// executeSQLScript-failure branch (runner.go) and executeSQLScript's own
// conn.Exec error branch (sql.go) together: a real SQL file with invalid SQL,
// run against the real testPool. MaxRetries: 0 keeps it to a single attempt.
func TestRunTaskWithRetry_ExecuteSQLScriptFailureIsWrapped(t *testing.T) {
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	cfg.RetryConfig = &config.RetryConfig{MaxRetries: 0, DeadlockRetries: 0}

	scriptPath := filepath.Join(t.TempDir(), "bad.sql")
	if err := os.WriteFile(scriptPath, []byte("THIS IS NOT VALID SQL;"), 0644); err != nil {
		t.Fatalf("failed to write SQL fixture: %v", err)
	}

	runner := process.NewRunner(cfg)
	task := process.NewTask("TEST: bad script", process.Params{}, scriptPath, 1, -1)

	err := runner.RunTaskWithRetry(task, testPool, cfg, 1)
	if err == nil {
		t.Fatal("expected an error for invalid SQL, got nil")
	}
	if !strings.Contains(err.Error(), scriptPath) {
		t.Errorf("expected the error to name the failing SQL file, got: %v", err)
	}
}

// TestWorkerStart_ContinuesPastFailedJob covers Worker.Start's error-continue
// branch: the first job's task fails for real (invalid SQL), the second job's
// task is real valid SQL that leaves a detectable side effect - proving Start
// drains the whole channel instead of exiting after the first failure.
func TestWorkerStart_ContinuesPastFailedJob(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	cfg.RetryConfig = &config.RetryConfig{MaxRetries: 0, DeadlockRetries: 0}

	const markerTable = "worker_continue_marker_test"
	if _, err := testPool.Exec(ctx, `DROP TABLE IF EXISTS `+markerTable); err != nil {
		t.Fatalf("failed to clean up marker table: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DROP TABLE IF EXISTS `+markerTable) })

	badScript := filepath.Join(t.TempDir(), "bad.sql")
	if err := os.WriteFile(badScript, []byte("THIS IS NOT VALID SQL;"), 0644); err != nil {
		t.Fatalf("failed to write bad SQL fixture: %v", err)
	}
	goodScript := filepath.Join(t.TempDir(), "good.sql")
	if err := os.WriteFile(goodScript, []byte(`CREATE TABLE `+markerTable+` (id int);`), 0644); err != nil {
		t.Fatalf("failed to write good SQL fixture: %v", err)
	}

	failingJob := process.NewJob([]int64{1}, []*process.Task{
		process.NewTask("TEST: failing", process.Params{}, badScript, 1, -1),
	})
	succeedingJob := process.NewJob([]int64{2}, []*process.Task{
		process.NewTask("TEST: succeeding", process.Params{}, goodScript, 1, -1),
	})

	queue := process.NewJobQueue()
	queue.Enqueue(failingJob)
	queue.Enqueue(succeedingJob)
	jobChan := queue.ToChannel()

	var wg sync.WaitGroup
	wg.Add(1)
	worker := process.NewWorker(1)
	worker.Start(jobChan, testPool, &wg, cfg)
	wg.Wait() // Start itself calls wg.Done() via defer; Wait just confirms it returned.

	var exists bool
	if err := testPool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, markerTable,
	).Scan(&exists); err != nil {
		t.Fatalf("failed to check marker table: %v", err)
	}
	if !exists {
		t.Error("expected the second job's task to have run despite the first job's failure — Start likely exited early instead of continuing")
	}
}

// TestGetBuildingObjectClassIDs_ScanErrorPropagates drives query.go's
// getBuildingObjectClassIDs rows.Scan error branch - not simulated: a NUMERIC
// column holding a fractional value (950.5) makes the query itself succeed
// (950.5 legitimately satisfies "BETWEEN 900 AND 999") while pgx's scan into
// a plain int genuinely fails, since a fractional value can't be losslessly
// converted.
func TestGetBuildingObjectClassIDs_ScanErrorPropagates(t *testing.T) {
	ctx := context.Background()
	const schema = "scan_error_objectclass_test"
	mustExec(t, ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	t.Cleanup(func() { testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	mustExec(t, ctx, `CREATE SCHEMA `+schema)
	mustExec(t, ctx, `CREATE TABLE `+schema+`.feature (objectclass_id NUMERIC)`)
	mustExec(t, ctx, `INSERT INTO `+schema+`.feature (objectclass_id) VALUES (950.5)`)

	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	cfg.DB.Schemas.Lod2 = schema

	err := process.RunFeatureExtraction(cfg, testPool)
	if err == nil {
		t.Fatal("expected a scan error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get LOD2 building IDs") {
		t.Errorf("expected RunFeatureExtraction's error wrap, got: %v", err)
	}
}

// TestGetProcessedBuildingFeatureIDs_ScanErrorPropagates drives query.go's
// getProcessedBuildingFeatureIDs rows.Scan error branch, reached from
// RunFeatureExtraction only after getBuildingIDsFromCityDB has already
// succeeded - so this needs real LOD2 CityDB data, plus a city2tabula
// _building table whose building_feature_id is a fractional NUMERIC.
//
// The already-processed check only queries rows that already exist, so a
// first, clean RunFeatureExtraction run is needed to populate
// city2tabula.lod2_building before corrupting a row and running again.
func TestGetProcessedBuildingFeatureIDs_ScanErrorPropagates(t *testing.T) {
	tc := setupGermanyLOD2(t)
	ctx := context.Background()
	cfg := pipelineConfig(*tc)

	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("initial RunFeatureExtraction (populates fixture data): %v", err)
	}

	mustExec(t, ctx, `ALTER TABLE city2tabula.lod2_building ALTER COLUMN building_feature_id TYPE NUMERIC`)
	t.Cleanup(func() {
		testPool.Exec(ctx, `ALTER TABLE city2tabula.lod2_building ALTER COLUMN building_feature_id TYPE INTEGER USING building_feature_id::INTEGER`)
	})
	// Only one row gets a fractional value - building_feature_id is UNIQUE, so
	// setting every row to the same 1.5 would fail the UPDATE itself before
	// scanning ever comes into play.
	mustExec(t, ctx, `UPDATE city2tabula.lod2_building SET building_feature_id = 1.5
		WHERE building_feature_id = (SELECT building_feature_id FROM city2tabula.lod2_building WHERE building_feature_id IS NOT NULL LIMIT 1)`)

	err := process.RunFeatureExtraction(cfg, testPool)
	if err == nil {
		t.Fatal("expected a scan error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "failed to filter already-processed LOD2 building IDs") {
		t.Errorf("expected RunFeatureExtraction's error wrap, got: %v", err)
	}
}

func mustExec(t *testing.T, ctx context.Context, sql string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, sql); err != nil {
		t.Fatalf("setup query failed (%s): %v", sql, err)
	}
}
