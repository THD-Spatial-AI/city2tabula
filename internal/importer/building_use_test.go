package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func TestParseBuildingUse(t *testing.T) {
	const header = "object_id,use_code,floor_area\n"
	rows, err := parseBuildingUse(strings.NewReader(header+"B1,woonfunctie,72\nB1,winkelfunctie,0.5\n"), "use.csv")
	if err != nil {
		t.Fatal(err)
	}
	want := []buildingUseRow{{"B1", "woonfunctie", 72}, {"B1", "winkelfunctie", 0.5}}
	if len(rows) != len(want) || rows[0] != want[0] || rows[1] != want[1] {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}

	tests := []struct{ name, data, wantErr string }{
		{"wrong header", "id,code,area\nB1,woonfunctie,1\n", "header is"},
		{"several codes in one row", header + "B1,\"woonfunctie,winkelfunctie\",10\n", "use.csv:2: use_code"},
		{"negative area", header + "B1,woonfunctie,-1\n", "use.csv:2: floor_area"},
		{"area not a number", header + "B1,woonfunctie,NaN\n", "use.csv:2: floor_area"},
		{"missing object id", header + ",woonfunctie,1\n", "use.csv:2: object_id"},
		{"missing column", header + "B1,woonfunctie\n", "wrong number of fields"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseBuildingUse(strings.NewReader(tc.data), "use.csv")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestDiscoverAttributeDatasets(t *testing.T) {
	attributes := t.TempDir()
	cfg := &config.Config{Data: &config.DataPaths{Attributes: attributes}}

	dir := writeDataset(t, attributes, "bag", "nl-bag-use")
	if _, err := DiscoverAttributeDatasets(cfg); err == nil || !strings.Contains(err.Error(), BuildingUseFile) {
		t.Errorf("folder without %s: error = %v, want one naming the file", BuildingUseFile, err)
	}

	if err := os.WriteFile(filepath.Join(dir, BuildingUseFile), []byte("object_id,use_code,floor_area\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	datasets, err := DiscoverAttributeDatasets(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(datasets) != 1 || datasets[0].Attribution.DatasetID != "nl-bag-use" {
		t.Errorf("datasets = %+v, want nl-bag-use", datasets)
	}

	for _, missing := range []string{"", filepath.Join(attributes, "absent")} {
		cfg.Data.Attributes = missing
		if datasets, err := DiscoverAttributeDatasets(cfg); err != nil || datasets != nil {
			t.Errorf("attributes directory %q: got %v, %v, want none and no error", missing, datasets, err)
		}
	}
}
