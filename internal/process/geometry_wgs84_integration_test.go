//go:build integration

package process_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// coordinates returns every position of a GeoJSON Polygon or MultiPolygon, and
// whether the object carries a crs member.
func coordinates(t *testing.T, raw json.RawMessage) ([][]float64, bool) {
	t.Helper()
	var g struct {
		Type        string          `json:"type"`
		CRS         json.RawMessage `json:"crs"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("geometry is not GeoJSON: %v\n%s", err, raw)
	}
	var out [][]float64
	switch g.Type {
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			t.Fatalf("polygon coordinates: %v", err)
		}
		for _, r := range rings {
			out = append(out, r...)
		}
	case "MultiPolygon":
		var polys [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			t.Fatalf("multipolygon coordinates: %v", err)
		}
		for _, p := range polys {
			for _, r := range p {
				out = append(out, r...)
			}
		}
	default:
		t.Fatalf("unexpected geometry type %q", g.Type)
	}
	return out, len(g.CRS) > 0
}

// TestServe_GeometryInWGS84 pins that the geometry query reprojects the footprint and
// every surface to EPSG:4326 on request: longitude and latitude around Deggendorf, no
// crs member (RFC 7946), the footprint 2D and the surfaces keeping their height.
// Without the option the output stays in the storage CRS, EPSG:25832.
func TestServe_GeometryInWGS84(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	building, _, _ := pylovoLinkFixtureBuildings(t, ctx)

	wgs, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{building}, true, true)
	if err != nil || len(wgs) != 1 {
		t.Fatalf("BuildingGeometryByObjectIDs(wgs84): %d buildings, %v", len(wgs), err)
	}
	inBavaria := func(p []float64) bool { return p[0] > 9 && p[0] < 14 && p[1] > 47 && p[1] < 51 }
	pts, hasCRS := coordinates(t, wgs[0].FootprintGeoJSON)
	if hasCRS {
		t.Error("WGS84 footprint carries a crs member, want none")
	}
	for _, p := range pts {
		if len(p) != 2 || !inBavaria(p) {
			t.Fatalf("WGS84 footprint position %v, want a 2D lon/lat in Bavaria", p)
		}
	}
	if len(wgs[0].Surfaces) == 0 {
		t.Fatal("no surfaces returned")
	}
	for _, s := range wgs[0].Surfaces {
		sp, _ := coordinates(t, s.GeoJSON)
		for _, p := range sp {
			if len(p) != 3 || !inBavaria(p) {
				t.Fatalf("WGS84 surface %s position %v, want lon/lat in Bavaria with height", s.ID, p)
			}
		}
	}

	native, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{building}, false, false)
	if err != nil || len(native) != 1 {
		t.Fatalf("BuildingGeometryByObjectIDs: %d buildings, %v", len(native), err)
	}
	pts, hasCRS = coordinates(t, native[0].FootprintGeoJSON)
	if !hasCRS || pts[0][0] < 100000 {
		t.Errorf("default footprint starts at %v (crs member %v), want EPSG:25832 metres with a crs member", pts[0], hasCRS)
	}
}
