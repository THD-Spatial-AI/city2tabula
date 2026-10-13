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

// TestServer_SurfaceDimensions extracts a gable house and checks the horizontal
// extent, slope extent and height served per surface. The house is 10 m by 8 m with
// 6 m walls and a 30 degree roof whose ridge runs along x, so each roof face is 10 m
// along the eave, 4/cos 30 m up the slope, and rises 4 tan 30 m.
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

	slope := 4 / math.Cos(math.Pi/6)
	want := map[string][]dims{
		"WallSurface": {
			{ptr(10), ptr(eave), eave}, {ptr(10), ptr(eave), eave},
			{ptr(8), ptr(ridge), ridge}, {ptr(8), ptr(ridge), ridge},
		},
		"RoofSurface":   {{ptr(10), ptr(slope), rise}, {ptr(10), ptr(slope), rise}},
		"GroundSurface": {{nil, nil, 0}},
	}
	got := map[string][]dims{}
	for _, b := range buildings {
		if b.ObjectID != "GABLE" {
			continue
		}
		for _, s := range b.Surfaces {
			if s.Height == nil {
				t.Fatalf("%s %s: height not served", s.Type, s.ID)
			}
			got[s.Type] = append(got[s.Type], dims{s.HorizontalExtent, s.SlopeExtent, *s.Height})
		}
	}
	for typ, ws := range want {
		if len(got[typ]) != len(ws) {
			t.Fatalf("%s: got %d surfaces %v, want %d", typ, len(got[typ]), got[typ], len(ws))
		}
		for _, w := range ws {
			if !hasDims(got[typ], w) {
				t.Errorf("%s: no surface %v in %v", typ, w, got[typ])
			}
		}
	}
}

// dims is one surface's served horizontal extent, slope extent and height. The
// extents are nil for flat faces.
type dims struct {
	horizontal, slope *float64
	height            float64
}

// String prints d with NULL for an absent extent.
func (d dims) String() string {
	return fmt.Sprintf("{horizontal %s slope %s height %.3f}", fmtPtr(d.horizontal), fmtPtr(d.slope), d.height)
}

// hasDims reports whether any of ds matches w within the 2-decimal rounding the
// pipeline stores.
func hasDims(ds []dims, w dims) bool {
	for _, d := range ds {
		if closeOrBothNil(d.horizontal, w.horizontal) && closeOrBothNil(d.slope, w.slope) && math.Abs(d.height-w.height) <= 0.01 {
			return true
		}
	}
	return false
}
