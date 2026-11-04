//go:build integration

package importer_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
)

// TestAddFootprints merges two adjacent tiles into one polygon, keeps a
// separate tile as a second one, ignores a file without an extent, and on a
// repeat call reads the extents from the catalogue cache, not the files.
func TestAddFootprints(t *testing.T) {
	ctx := context.Background()
	if err := importer.PrepareCatalogue(ctx, testPool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DROP TABLE IF EXISTS public.c2t_file_extent`) })

	lod2 := t.TempDir()
	dataset := filepath.Join(lod2, "nrw")
	writeAttributionFolder(t, dataset, "xx-nrw-lod2", "Credit")
	tiles := map[string]string{
		"a.gml":      tileGML(356000, 5644000),
		"b.gml":      tileGML(358000, 5644000),
		"far.gml":    tileGML(370000, 5644000),
		"nodecl.gml": "<core:CityModel/>",
	}
	for name, body := range tiles {
		if err := os.WriteFile(filepath.Join(dataset, "gml", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := testConfig()
	cfg.Data = &config.DataPaths{Lod2: lod2, Lod3: filepath.Join(lod2, "missing-lod3")}

	want := [][4]float64{
		bounds(t, bboxIn4326(t, ctx, 356000, 5644000, 360000, 5646000)),
		bounds(t, bboxIn4326(t, ctx, 370000, 5644000, 372000, 5646000)),
	}
	for _, call := range []string{"first", "cached"} {
		list, err := importer.ListDatasets(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := importer.AddFootprints(testPool, list); err != nil {
			t.Fatalf("%s call: %v", call, err)
		}
		var fp struct {
			Type        string          `json:"type"`
			Coordinates [][][][]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(list[0].Footprint, &fp); err != nil {
			t.Fatalf("%s call: footprint %s: %v", call, list[0].Footprint, err)
		}
		if fp.Type != "MultiPolygon" || len(fp.Coordinates) != len(want) {
			t.Fatalf("%s call: footprint %s, want a MultiPolygon of %d polygons", call, list[0].Footprint, len(want))
		}
		sort.Slice(fp.Coordinates, func(i, j int) bool {
			return ringBounds(fp.Coordinates[i][0])[0] < ringBounds(fp.Coordinates[j][0])[0]
		})
		for i, polygon := range fp.Coordinates {
			if got := ringBounds(polygon[0]); !near(got, want[i], 0.002) {
				t.Errorf("%s call: polygon %d spans %v, want about %v", call, i, got, want[i])
			}
		}
		// Unreadable files prove the second call takes its extents from the cache.
		for name := range tiles {
			if err := os.Chmod(filepath.Join(dataset, "gml", name), 0); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// bounds parses an import bbox "xmin,ymin,xmax,ymax,srid".
func bounds(t *testing.T, bbox string) [4]float64 {
	t.Helper()
	var b [4]float64
	for i, s := range strings.Split(bbox, ",")[:4] {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		b[i] = v
	}
	return b
}

// ringBounds returns the [min_lon, min_lat, max_lon, max_lat] of a ring.
func ringBounds(ring [][]float64) [4]float64 {
	b := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, p := range ring {
		b[0], b[1] = math.Min(b[0], p[0]), math.Min(b[1], p[1])
		b[2], b[3] = math.Max(b[2], p[0]), math.Max(b[3], p[1])
	}
	return b
}

// near reports whether every value of a lies within tol of b's.
func near(a, b [4]float64, tol float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > tol {
			return false
		}
	}
	return true
}
