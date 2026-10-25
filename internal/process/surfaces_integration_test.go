//go:build integration

package process_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// TestSurfacesInBBox_PagesCoverEveryBuildingOnce pins the bulk surfaces path on
// the real Bavarian seed: paging with a small vertex budget returns every
// building of the bbox exactly once, each page within the budget unless it holds
// a single building, roof and wall surfaces only, as 3D WGS84 polygons carrying
// the same ids as the geometry endpoint.
func TestSurfacesInBBox_PagesCoverEveryBuildingOnce(t *testing.T) {
	cfg, _ := setupCorrectionAuditFixture(t)
	ctx := context.Background()
	bbox := onrequest.Bbox{Xmin: 5, Ymin: 47, Xmax: 15, Ymax: 55}

	want, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bbox)
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	var wantIDs []string
	for _, b := range want {
		wantIDs = append(wantIDs, b.ObjectID)
	}
	slices.Sort(wantIDs)
	wantIDs = slices.Compact(wantIDs)

	const budget = 300
	var gotIDs []string
	byID := map[string]onrequest.BuildingSurfaces{}
	after, pages := "", 0
	for {
		page, err := onrequest.SurfacesInBBox(ctx, testPool, cfg, bbox, after, budget)
		if err != nil {
			t.Fatalf("SurfacesInBBox page %d: %v", pages+1, err)
		}
		pages++
		if pages > len(wantIDs)+1 {
			t.Fatalf("still paging after %d pages for %d buildings; the cursor does not advance", pages, len(wantIDs))
		}
		if len(page.Buildings) == 0 {
			t.Fatalf("page %d is empty", pages)
		}
		vertices := 0
		for _, b := range page.Buildings {
			gotIDs = append(gotIDs, b.ObjectID)
			byID[b.ObjectID] = b
			vertices += checkRenderSurfaces(t, b)
		}
		if vertices > budget && len(page.Buildings) > 1 {
			t.Errorf("page %d holds %d buildings and %d vertices, over the budget of %d", pages, len(page.Buildings), vertices, budget)
		}
		if page.NextCursor == "" {
			break
		}
		if after, err = onrequest.DecodeSurfaceCursor(page.NextCursor); err != nil {
			t.Fatalf("next cursor: %v", err)
		}
	}
	if pages < 2 {
		t.Errorf("got %d page with a budget of %d vertices; the fixture should need several", pages, budget)
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("paged buildings (%d) differ from the bbox's buildings (%d)", len(gotIDs), len(wantIDs))
	}

	buildingID := ""
	for _, id := range gotIDs {
		if len(byID[id].Surfaces) > 0 {
			buildingID = id
			break
		}
	}
	geometry, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{buildingID}, true, true)
	if err != nil || len(geometry) == 0 {
		t.Fatalf("BuildingGeometryByObjectIDs(%s): %v", buildingID, err)
	}
	var geometryIDs []string
	for _, s := range geometry[0].Surfaces {
		if s.Type == "RoofSurface" || s.Type == "WallSurface" {
			geometryIDs = append(geometryIDs, s.ID)
		}
	}
	var surfaceIDs []string
	for _, s := range byID[buildingID].Surfaces {
		surfaceIDs = append(surfaceIDs, s.ID)
	}
	slices.Sort(geometryIDs)
	slices.Sort(surfaceIDs)
	if len(surfaceIDs) == 0 || !slices.Equal(surfaceIDs, geometryIDs) {
		t.Errorf("%s: surface ids %v differ from the geometry endpoint's %v", buildingID, surfaceIDs, geometryIDs)
	}
}

// checkRenderSurfaces checks one building's surfaces and returns their vertex count.
func checkRenderSurfaces(t *testing.T, b onrequest.BuildingSurfaces) int {
	t.Helper()
	vertices := 0
	for _, s := range b.Surfaces {
		if s.ID == "" || (s.Type != "RoofSurface" && s.Type != "WallSurface") {
			t.Errorf("%s: surface %q of type %q", b.ObjectID, s.ID, s.Type)
		}
		var g struct {
			Type        string        `json:"type"`
			Coordinates [][][]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(s.Geometry, &g); err != nil || g.Type != "Polygon" {
			t.Fatalf("%s surface %s: geometry %s is not a Polygon: %v", b.ObjectID, s.ID, s.Geometry, err)
		}
		for _, ring := range g.Coordinates {
			vertices += len(ring)
			for _, p := range ring {
				if len(p) != 3 || p[0] < 5 || p[0] > 15 || p[1] < 47 || p[1] > 55 {
					t.Fatalf("%s surface %s: point %v is not a WGS84 point with z in Bavaria", b.ObjectID, s.ID, p)
				}
			}
		}
	}
	return vertices
}
