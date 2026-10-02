//go:build integration

package process_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// A building without a usable dataset would make script 04's NOT NULL foreign
// key reject its whole batch, and a failed task does not fail the run. The
// check before extraction must stop the run instead, naming the cause, before
// any building row is written.
func TestRunFeatureExtraction_RefusesBuildingsWithoutDataset(t *testing.T) {
	const seed = "testdata/germany/seed_lod2.sql"
	if _, err := os.Stat(seed); err != nil {
		t.Skipf("seed file not found, skipping: %s", seed)
	}
	tests := []struct {
		name, lineage string
		want          []string
	}{
		{"no lineage", "NULL", []string{"no dataset lineage", "object id "}},
		{"unregistered dataset", "'unregistered'", []string{`"unregistered"`, "-sync-attribution"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			resetSchemas(t)
			cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
			if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
				t.Fatalf("RunCity2TabulaDBSetup: %v", err)
			}
			seedDB(t, seed)

			var objectID string
			if err := testPool.QueryRow(ctx, `
				UPDATE lod2.feature SET lineage = `+tc.lineage+`
				WHERE id = (SELECT min(id) FROM lod2.feature WHERE objectclass_id = 901)
				RETURNING objectid`).Scan(&objectID); err != nil {
				t.Fatalf("break one building's lineage: %v", err)
			}

			err := process.RunFeatureExtraction(cfg, testPool)
			if err == nil {
				t.Fatal("RunFeatureExtraction returned nil for a building without a usable dataset")
			}
			want := tc.want
			if tc.lineage == "NULL" {
				want = append(want, objectID)
			}
			for _, w := range want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}

			var rows int
			if err := testPool.QueryRow(ctx, `SELECT count(*) FROM city2tabula.lod2_building`).Scan(&rows); err != nil {
				t.Fatalf("count lod2_building: %v", err)
			}
			if rows != 0 {
				t.Errorf("expected no building rows after a refused run, got %d", rows)
			}
		})
	}
}
