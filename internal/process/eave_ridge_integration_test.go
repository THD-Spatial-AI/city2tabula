//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestPipeline_GableHouse_EaveAndRidge pins eave and ridge height to the roof. The
// house is 10 m by 8 m with 6 m walls and a 30 degree gable roof, so the ridge is at
// 6 + 4 tan 30 m. Its gable walls reach the ridge and must not be read as the eave.
func TestPipeline_GableHouse_EaveAndRidge(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const x, y, z0, eave = -8000.0, 342800.0, 200.0, 6.0
	ridge := eave + 4*math.Tan(math.Pi/6)
	f := &partsFixture{nextID: 500000}
	bld := f.feature(901, "GABLE")
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), bld)
	e, r := z0+eave, z0+ridge
	for _, s := range []struct {
		class int
		ring  [][3]float64
	}{
		{710, [][3]float64{{x, y, z0}, {x, y + 8, z0}, {x + 10, y + 8, z0}, {x + 10, y, z0}}},
		{709, [][3]float64{{x, y, z0}, {x + 10, y, z0}, {x + 10, y, e}, {x, y, e}}},
		{709, [][3]float64{{x + 10, y + 8, z0}, {x, y + 8, z0}, {x, y + 8, e}, {x + 10, y + 8, e}}},
		{709, [][3]float64{{x + 10, y, z0}, {x + 10, y + 8, z0}, {x + 10, y + 8, e}, {x + 10, y + 4, r}, {x + 10, y, e}}},
		{709, [][3]float64{{x, y + 8, z0}, {x, y, z0}, {x, y, e}, {x, y + 4, r}, {x, y + 8, e}}},
		{712, [][3]float64{{x, y, e}, {x + 10, y, e}, {x + 10, y + 4, r}, {x, y + 4, r}}},
		{712, [][3]float64{{x + 10, y + 8, e}, {x, y + 8, e}, {x, y + 4, r}, {x + 10, y + 4, r}}},
	} {
		f.surface(bld, s.class, s.ring)
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var minH, maxH float64
	if err := testPool.QueryRow(ctx, `
		SELECT min_height, max_height FROM city2tabula.lod2_building WHERE object_id = 'GABLE'`,
	).Scan(&minH, &maxH); err != nil {
		t.Fatalf("read GABLE heights: %v", err)
	}
	if math.Abs(minH-eave) > 0.01 || math.Abs(maxH-ridge) > 0.01 {
		t.Errorf("GABLE: min_height %.2f, max_height %.2f; want eave %.2f and ridge %.2f", minH, maxH, eave, ridge)
	}
}
