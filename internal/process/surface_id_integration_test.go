//go:build integration

package process_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// surfaceIDs returns the served surface ids of one building, sorted.
func surfaceIDs(t *testing.T, ctx context.Context, objectID string) []string {
	t.Helper()
	rows, err := testPool.Query(ctx,
		`SELECT id::text FROM city2tabula.lod2_surface WHERE building_object_id = $1 ORDER BY 1`, objectID)
	if err != nil {
		t.Fatalf("read surface ids of %s: %v", objectID, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan surface id of %s: %v", objectID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read surface ids of %s: %v", objectID, err)
	}
	return ids
}

// TestPipeline_SurfaceIDsSurviveRebuild pins that a served surface keeps its id when a
// later extraction rebuilds the surface table, and when the whole database is extracted
// again from the same source: the id derives from the source, not from row order.
func TestPipeline_SurfaceIDsSurviveRebuild(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	const y, z0, z1 = 342800.0, 200.0, 210.0
	first := &partsFixture{nextID: 810000}
	first.solid(first.feature(901, "FIRST"), box{-4000, y, -3990, y + 10, z0, z1, "SENW", z0})

	extractFirst := func() []string {
		resetSchemas(t)
		if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
			t.Fatalf("RunCity2TabulaDBSetup: %v", err)
		}
		mustExec(t, ctx, buildPartsFixture()+first.sql.String())
		seedDB(t)
		if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
			t.Fatalf("RunFeatureExtraction: %v", err)
		}
		ids := surfaceIDs(t, ctx, "FIRST")
		if len(ids) == 0 {
			t.Fatal("extraction served no surfaces for FIRST")
		}
		return ids
	}

	before := extractFirst()
	if again := extractFirst(); !slices.Equal(before, again) {
		t.Errorf("FIRST's surface ids differ after extracting from scratch again:\nfirst  %v\nsecond %v", before, again)
	}

	// A building far from FIRST: extracting it re-runs the post scripts, which
	// rebuild every served surface, FIRST's included.
	second := &partsFixture{nextID: 820000}
	second.solid(second.feature(901, "SECOND"), box{-3000, y, -2990, y + 10, z0, z1, "SENW", z0})
	mustExec(t, ctx, second.sql.String()+
		"UPDATE lod2.feature SET lineage = 'test-dataset' WHERE lineage IS NULL;")
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("second RunFeatureExtraction: %v", err)
	}

	if after := surfaceIDs(t, ctx, "FIRST"); !slices.Equal(before, after) {
		t.Errorf("FIRST's surface ids changed on rebuild:\nbefore %v\nafter  %v", before, after)
	}
}
