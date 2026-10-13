//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
)

// TestSurfaceDimensions pins surface_dimensions to faces with known extents
// along the horizontal and the fall line. The narrow wall (2 m wide, 8 m tall)
// checks that its 8 m vertical side lands in slope_extent, not horizontal_extent.
func TestSurfaceDimensions(t *testing.T) {
	ctx := context.Background()
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	// nil wants mark the flat faces, which have no horizontal axis in their plane.
	cases := []struct {
		name              string
		wkt               string
		nx, ny, nz        float64
		horizontal, slope *float64
	}{
		{"wall 10 wide 3 tall", "POLYGON Z((0 0 0, 10 0 0, 10 0 3, 0 0 3, 0 0 0))",
			0, -1, 0, ptr(10), ptr(3)},
		{"wall 2 wide 8 tall", "POLYGON Z((0 0 0, 2 0 0, 2 0 8, 0 0 8, 0 0 0))",
			0, -1, 0, ptr(2), ptr(8)},
		{"wall normal pointing inward", "POLYGON Z((0 0 0, 10 0 0, 10 0 3, 0 0 3, 0 0 0))",
			0, 1, 0, ptr(10), ptr(3)},
		{"wall 6 wide rotated 30 degrees in plan", "POLYGON Z((0 0 0, 5.196152 3 0, 5.196152 3 3, 0 0 3, 0 0 0))",
			0.5, -math.Sqrt(3) / 2, 0, ptr(6), ptr(3)},
		{"roof south 30 degree pitch", "POLYGON Z((0 0 0, 12 0 0, 12 5 2.886751, 0 5 2.886751, 0 0 0))",
			0, -0.5, math.Sqrt(3) / 2, ptr(12), ptr(5.77)},
		{"roof east 30 degree pitch", "POLYGON Z((0 0 2.886751, 5 0 0, 5 12 0, 0 12 2.886751, 0 0 2.886751))",
			0.5, 0, math.Sqrt(3) / 2, ptr(12), ptr(5.77)},
		{"gable wall base 8 apex 3", "POLYGON Z((0 0 0, 8 0 0, 4 0 3, 0 0 0))",
			0, -1, 0, ptr(8), ptr(3)},
		{"collinear sliver", "POLYGON Z((0 0 0, 5 0 0, 10 0 0, 0 0 0))",
			0, -1, 0, ptr(10), ptr(0)},
		{"flat roof 8 x 4", "POLYGON Z((0 0 5, 8 0 5, 8 4 5, 0 4 5, 0 0 5))",
			0, 0, 1, nil, nil},
		{"ground face", "POLYGON Z((0 0 0, 0 4 0, 8 4 0, 8 0 0, 0 0 0))",
			0, 0, -1, nil, nil},
	}

	for _, c := range cases {
		var horizontal, slope *float64
		err := testPool.QueryRow(ctx,
			`SELECT horizontal_extent, slope_extent FROM city2tabula.surface_dimensions(
				ST_GeomFromText($1, 25832), $2, $3, $4)`,
			c.wkt, c.nx, c.ny, c.nz,
		).Scan(&horizontal, &slope)
		if err != nil {
			t.Fatalf("%s: surface_dimensions: %v", c.name, err)
		}
		if !closeOrBothNil(horizontal, c.horizontal) || !closeOrBothNil(slope, c.slope) {
			t.Errorf("%s: got horizontal %s slope %s, want %s x %s",
				c.name, fmtPtr(horizontal), fmtPtr(slope), fmtPtr(c.horizontal), fmtPtr(c.slope))
		}
	}
}

func ptr(v float64) *float64 { return &v }

// closeOrBothNil reports whether got and want are both NULL, or both set and within 0.01 m.
func closeOrBothNil(got, want *float64) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return math.Abs(*got-*want) <= 0.01
}

func fmtPtr(v *float64) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%.3f", *v)
}
