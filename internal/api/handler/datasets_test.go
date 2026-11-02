package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
)

const testAttribution = `{"schema_version": 1, "dataset_id": "%s", "provider": "Example Provider",
	"dataset": "Example buildings", "licence": "CC-BY-4.0",
	"licence_url": "https://creativecommons.org/licenses/by/4.0/",
	"credit": "Example Provider", "changes": "None."}`

// writeTestFile writes content to path under the working directory.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDatasets lists a German dataset split into regions and a Dutch one
// without regions.json, from a data folder with no database behind it. A
// folder not named after a supported country is skipped.
func TestDatasets(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, "data/lod2/germany/nrw/attribution.json", strings.Replace(testAttribution, "%s", "de-nrw-lod2", 1))
	writeTestFile(t, "data/lod2/germany/nrw/regions.json",
		`{"schema_version": 1, "regions": {"koeln": {"name": "Köln, Nordrhein-Westfalen", "bbox": [6.77, 50.83, 7.16, 51.08]}}}`)
	writeTestFile(t, "data/lod2/germany/nrw/gml/koeln/a.gml.gz", "")
	writeTestFile(t, "data/lod2/germany/nrw/gml/koeln/b.gml", "")
	writeTestFile(t, "data/lod2/netherlands/3dbag/attribution.json", strings.Replace(testAttribution, "%s", "nl-3dbag-lod2", 1))
	writeTestFile(t, "data/lod2/netherlands/3dbag/cityjson/tile.city.json", "")
	writeTestFile(t, "data/lod2/germany.parked/x/attribution.json", "not a country folder")

	h := New(server.New(config.Config{
		DB:     &config.DBConfig{Name: "c2t", Schemas: &config.Schemas{Lod2: "lod2", Lod3: "lod3"}},
		Data:   &config.DataPaths{},
		CityDB: &config.CityDB{},
	}))
	tests := []struct {
		query    string
		status   int
		datasets []importer.DatasetRegion
	}{
		{"", http.StatusOK, []importer.DatasetRegion{
			{Country: "germany", DatasetID: "de-nrw-lod2", Provider: "Example Provider", LOD: 2, Format: "citygml",
				Region: "koeln", Name: "Köln, Nordrhein-Westfalen", Bbox: []float64{6.77, 50.83, 7.16, 51.08}, Files: 2},
			{Country: "netherlands", DatasetID: "nl-3dbag-lod2", Provider: "Example Provider", LOD: 2, Format: "cityjson", Files: 1},
		}},
		{"?country=netherlands", http.StatusOK, []importer.DatasetRegion{
			{Country: "netherlands", DatasetID: "nl-3dbag-lod2", Provider: "Example Provider", LOD: 2, Format: "cityjson", Files: 1},
		}},
		{"?country=austria", http.StatusOK, []importer.DatasetRegion{}},
		{"?country=atlantis", http.StatusBadRequest, nil},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.Datasets(rec, httptest.NewRequest(http.MethodGet, "/api/v1/datasets"+tc.query, nil))
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if tc.datasets == nil {
				return
			}
			want, _ := json.Marshal(map[string]any{"datasets": tc.datasets})
			if got := strings.TrimSpace(rec.Body.String()); got != string(want) {
				t.Errorf("body\n%s\nwant\n%s", got, want)
			}
		})
	}
}
