//go:build integration

package process_test

import (
	"context"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// TestSurfaceDimensions pins surface_dimensions to faces with known in-plane
// sides. The pitched roof is the case a plan-view measurement gets wrong: its
// slope side is 5 m in plan but 5.77 m in the roof plane.
func TestSurfaceDimensions(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	cases := []struct {
		name          string
		wkt           string
		nx, ny, nz    float64
		length, width float64
	}{
		{"wall 10 x 3", "POLYGON Z((0 0 0, 10 0 0, 10 0 3, 0 0 3, 0 0 0))",
			0, -1, 0, 10, 3},
		{"roof 30 degree pitch", "POLYGON Z((0 0 0, 12 0 0, 12 5 2.886751, 0 5 2.886751, 0 0 0))",
			0, -0.5, math.Sqrt(3) / 2, 12, 5.77},
		{"flat 8 x 4 rotated 30 degrees in plan", "POLYGON Z((0 0 5, 6.928203 4 5, 4.928203 7.464102 5, -2 3.464102 5, 0 0 5))",
			0, 0, 1, 8, 4},
		{"obtuse gable base 8 rise 1", "POLYGON Z((0 0 0, 8 0 0, 4 0 1, 0 0 0))",
			0, -1, 0, 8, 1},
		{"collinear sliver", "POLYGON Z((0 0 0, 5 0 0, 10 0 0, 0 0 0))",
			0, -1, 0, 10, 0},
	}

	for _, c := range cases {
		var length, width float64
		err := testPool.QueryRow(ctx,
			`SELECT length, width FROM city2tabula.surface_dimensions(
				ST_GeomFromText($1, 25832), $2, $3, $4)`,
			c.wkt, c.nx, c.ny, c.nz,
		).Scan(&length, &width)
		if err != nil {
			t.Fatalf("%s: surface_dimensions: %v", c.name, err)
		}
		if math.Abs(length-c.length) > 0.01 || math.Abs(width-c.width) > 0.01 {
			t.Errorf("%s: got length %.3f width %.3f, want %.2f x %.2f",
				c.name, length, width, c.length, c.width)
		}
	}
}
