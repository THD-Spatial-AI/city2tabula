//go:build integration

package process_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// TestServer_SurfaceDimensions extracts a gable house and checks the length, width
// and height served per surface. The house is 10 m by 8 m with 6 m walls and a 30
// degree roof whose ridge runs along x, so each roof face is 10 m by 4/cos 30 m in
// its own plane and rises 4 tan 30 m.
func TestServer_SurfaceDimensions(t *testing.T) {
	ctx := context.Background()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "austria", srid: fmt.Sprint(partsSRID)})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	const x, y, eave = -8000.0, 342800.0, 6.0
	rise := 4 * math.Tan(math.Pi/6)
	ridge := eave + rise
	f := &partsFixture{nextID: 400000}
	bld := f.feature(901, "GABLE")
	fmt.Fprintf(&f.sql, "INSERT INTO lod2.property (id, feature_id, name, val_lod) VALUES (%d, %d, 'lod2Solid', '2');\n", f.id(), bld)
	for _, s := range []struct {
		class int
		ring  [][3]float64
	}{
		{710, [][3]float64{{x, y, 0}, {x, y + 8, 0}, {x + 10, y + 8, 0}, {x + 10, y, 0}}},
		{709, [][3]float64{{x, y, 0}, {x + 10, y, 0}, {x + 10, y, eave}, {x, y, eave}}},
		{709, [][3]float64{{x + 10, y + 8, 0}, {x, y + 8, 0}, {x, y + 8, eave}, {x + 10, y + 8, eave}}},
		{709, [][3]float64{{x + 10, y, 0}, {x + 10, y + 8, 0}, {x + 10, y + 8, eave}, {x + 10, y + 4, ridge}, {x + 10, y, eave}}},
		{709, [][3]float64{{x, y + 8, 0}, {x, y, 0}, {x, y, eave}, {x, y + 4, ridge}, {x, y + 8, eave}}},
		{712, [][3]float64{{x, y, eave}, {x + 10, y, eave}, {x + 10, y + 4, ridge}, {x, y + 4, ridge}}},
		{712, [][3]float64{{x + 10, y + 8, eave}, {x, y + 8, eave}, {x, y + 4, ridge}, {x + 10, y + 4, ridge}}},
	} {
		f.surface(bld, s.class, s.ring)
	}
	mustExec(t, ctx, buildPartsFixture()+f.sql.String())
	seedDB(t)
	if err := process.RunFeatureExtraction(cfg, testPool); err != nil {
		t.Fatalf("RunFeatureExtraction: %v", err)
	}

	var bbox onrequest.Bbox
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, $5), 4326) AS e) s`,
		x, y, x+10, y+8, partsSRID,
	).Scan(&bbox.Xmin, &bbox.Ymin, &bbox.Xmax, &bbox.Ymax); err != nil {
		t.Fatalf("build bbox: %v", err)
	}
	buildings, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bbox)
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}

	want := map[string][]dims{
		"WallSurface":   {{10, eave, eave}, {10, eave, eave}, {ridge, 8, ridge}, {ridge, 8, ridge}},
		"RoofSurface":   {{10, 4 / math.Cos(math.Pi/6), rise}, {10, 4 / math.Cos(math.Pi/6), rise}},
		"GroundSurface": {{10, 8, 0}},
	}
	got := map[string][]dims{}
	for _, b := range buildings {
		if b.ObjectID != "GABLE" {
			continue
		}
		for _, s := range b.Surfaces {
			if s.Length == nil || s.Width == nil || s.Height == nil {
				t.Fatalf("%s %s: length, width or height not served", s.Type, s.ID)
			}
			got[s.Type] = append(got[s.Type], dims{*s.Length, *s.Width, *s.Height})
		}
	}
	for typ, ws := range want {
		if len(got[typ]) != len(ws) {
			t.Fatalf("%s: got %d surfaces %v, want %d", typ, len(got[typ]), got[typ], len(ws))
		}
		for _, w := range ws {
			if !hasDims(got[typ], w) {
				t.Errorf("%s: no surface with length %.2f, width %.2f, height %.2f in %v", typ, w.length, w.width, w.height, got[typ])
			}
		}
	}
}

// dims is one surface's served length, width and height.
type dims struct{ length, width, height float64 }

// hasDims reports whether any of ds matches w within the 2-decimal rounding the
// pipeline stores.
func hasDims(ds []dims, w dims) bool {
	for _, d := range ds {
		if math.Abs(d.length-w.length) <= 0.01 && math.Abs(d.width-w.width) <= 0.01 && math.Abs(d.height-w.height) <= 0.01 {
			return true
		}
	}
	return false
}
