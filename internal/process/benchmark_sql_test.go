//go:build integration

package process_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// sqlParams are the fixed parameter values used in all SQL benchmarks.
// They match integrationConfig() and the seed data in testdata/germany/.
var sqlParams = map[string]string{
	"{lod_schema}":           "lod2",
	"{lod_level}":            "2",
	"{srid}":                 "25832",
	"{city2tabula_schema}":   "city2tabula",
	"{tabula_schema}":        "tabula",
	"{public_schema}":        "public",
	"{citydb_schema}":        "citydb",
	"{citydb_pkg_schema}":    "citydb_pkg",
	"{country}":              "germany",
	"{tabula_table}":         "tabula",
	"{tabula_variant_table}": "tabula_variant",
	"{room_height}":          "2.5",
	"{building_ids}":         "(1,8)",
}

func applyParams(script string) string {
	for k, v := range sqlParams {
		script = strings.ReplaceAll(script, k, v)
	}
	return script
}

var benchSetupOnce sync.Once

// setupBenchmarkDB creates the city2tabula schema and seeds test data exactly once
// across all benchmark functions in this file. Subsequent calls are no-ops.
func setupBenchmarkDB(b *testing.B) {
	b.Helper()
	benchSetupOnce.Do(func() {
		ctx := context.Background()
		cfg := integrationConfig()

		if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
			b.Fatalf("RunCity2TabulaDBSetup: %v", err)
		}

		for _, path := range []string{
			"testdata/germany/seed_lod2.sql",
			"testdata/germany/seed_tabula_variant.sql",
		} {
			sql, err := os.ReadFile(path)
			if err != nil {
				b.Fatalf("failed to read %s: %v", path, err)
			}
			if _, err := testPool.Exec(ctx, string(sql)); err != nil {
				b.Fatalf("failed to seed %s: %v", path, err)
			}
		}
	})
}

// truncateOutputTables resets all city2tabula output tables between benchmark iterations
// so each loop iteration measures the same amount of work.
func truncateOutputTables(b *testing.B) {
	b.Helper()
	_, err := testPool.Exec(context.Background(), `
		TRUNCATE city2tabula.lod2_building_feature CASCADE;
		TRUNCATE city2tabula.lod2_child_feature_surface CASCADE;
		TRUNCATE city2tabula.lod2_child_feature_geom_dump CASCADE;
		TRUNCATE city2tabula.lod2_child_feature CASCADE;
	`)
	if err != nil {
		b.Fatalf("failed to truncate output tables: %v", err)
	}
}

// runScriptBenchmark is the shared benchmark driver.
// It reads a SQL script, substitutes parameters, then in each iteration:
//   - stops the timer to reset output tables (setup cost excluded)
//   - restarts the timer and executes the script
func runScriptBenchmark(b *testing.B, scriptPath string) {
	b.Helper()
	ctx := context.Background()

	raw, err := os.ReadFile(scriptPath)
	if err != nil {
		b.Fatalf("failed to read %s: %v", scriptPath, err)
	}
	script := applyParams(string(raw))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		truncateOutputTables(b)
		b.StartTimer()

		if _, err := testPool.Exec(ctx, script); err != nil {
			b.Fatalf("script %s failed: %v", scriptPath, err)
		}
	}
}

// =============================================================================
// Per-script benchmarks
// Each benchmark measures a single SQL script's execution time against 2
// synthetic LoD2 buildings. Run all with:
//
//	go test -tags integration -bench=BenchmarkScript -benchmem -run=^$ ./internal/process/
//
// =============================================================================

func BenchmarkScript_01_GetChildFeat(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/01_get_child_feat.sql")
}

func BenchmarkScript_02_DumpChildFeatGeom(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/02_dump_child_feat_geom.sql")
}

func BenchmarkScript_03_CalcChildFeatAttr(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/03_calc_child_feat_attr.sql")
}

func BenchmarkScript_04_CalcBldFeat(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/04_calc_bld_feat.sql")
}

func BenchmarkScript_05_CalcVolume(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/05_calc_volume.sql")
}

func BenchmarkScript_06_CalcStoreys(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/06_calc_storeys.sql")
}

func BenchmarkScript_07_LabelBuildingFeatures(b *testing.B) {
	setupBenchmarkDB(b)
	runScriptBenchmark(b, "sql/scripts/main/07_label_building_features.sql")
}
