package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// writeDataset creates lodDir/name holding a valid attribution.json for
// datasetID and empty gml and cityjson model folders, and returns the dataset
// folder's path.
func writeDataset(t *testing.T, lodDir, name, datasetID string) string {
	t.Helper()
	dir := filepath.Join(lodDir, name)
	for _, sub := range []string{"gml", "cityjson"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("create dataset folder: %v", err)
		}
	}
	data, err := json.Marshal(attribution.Attribution{
		SchemaVersion: attribution.SchemaVersion,
		DatasetID:     datasetID,
		Provider:      "Example Provider",
		Dataset:       "Example buildings",
		Licence:       "CC-BY-4.0",
		LicenceURL:    "https://creativecommons.org/licenses/by/4.0/",
		Credit:        "Example Provider, example.org",
		Changes:       "City2TABULA derives building attributes from the geometry.",
	})
	if err != nil {
		t.Fatalf("marshal attribution: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, attribution.FileName), data, 0o644); err != nil {
		t.Fatalf("write attribution: %v", err)
	}
	return dir
}

func discoverConfig(lod2Dir, lod3Dir string) *config.Config {
	return &config.Config{
		Data: &config.DataPaths{Lod2: lod2Dir, Lod3: lod3Dir},
		DB:   &config.DBConfig{Schemas: &config.Schemas{Lod2: "lod2", Lod3: "lod3"}},
	}
}

func TestDiscoverDatasets_FindsFoldersInBothLODs(t *testing.T) {
	lod2Dir, lod3Dir := t.TempDir(), t.TempDir()
	a := writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	b := writeDataset(t, lod2Dir, "region-b", "xx-region-b-lod2")
	c := writeDataset(t, lod3Dir, "region-a", "xx-region-a-lod3")
	// Neither a non-model file nor a hidden folder is a dataset.
	if err := os.WriteFile(filepath.Join(lod2Dir, "README.md"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(lod2Dir, ".cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverDatasets(discoverConfig(lod2Dir, lod3Dir))
	if err != nil {
		t.Fatalf("DiscoverDatasets: %v", err)
	}

	want := map[string]Dataset{
		"xx-region-a-lod2": {Dir: a, Schema: "lod2"},
		"xx-region-b-lod2": {Dir: b, Schema: "lod2"},
		"xx-region-a-lod3": {Dir: c, Schema: "lod3"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d datasets, want %d: %+v", len(got), len(want), got)
	}
	for _, d := range got {
		w, ok := want[d.Attribution.DatasetID]
		inputs := []importInput{
			{"citygml", "CityGML", filepath.Join(w.Dir, "gml")},
			{"cityjson", "CityJSON", filepath.Join(w.Dir, "cityjson")},
		}
		if !ok || d.Dir != w.Dir || d.Schema != w.Schema || !slices.Equal(d.Inputs, inputs) {
			t.Errorf("unexpected dataset %s in %s (schema %s)", d.Attribution.DatasetID, d.Dir, d.Schema)
		}
	}
}

func TestDiscoverDatasets_MissingLODDirSkipped(t *testing.T) {
	lod2Dir := t.TempDir()
	writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")

	got, err := DiscoverDatasets(discoverConfig(lod2Dir, filepath.Join(t.TempDir(), "absent")))
	if err != nil {
		t.Fatalf("DiscoverDatasets: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d datasets, want 1: %+v", len(got), got)
	}
}

func TestDiscoverDatasets_RejectsModelFileOutsideDatasetFolder(t *testing.T) {
	for _, name := range []string{"tile.gml", "tile.city.json", "TILE.XML", "tile.jsonl"} {
		t.Run(name, func(t *testing.T) {
			lod2Dir := t.TempDir()
			writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
			path := filepath.Join(lod2Dir, name)
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := DiscoverDatasets(discoverConfig(lod2Dir, t.TempDir()))
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Errorf("want an error naming %s, got %v", path, err)
			}
		})
	}
}

// Source material beside the data, such as a 3D Tiles index, a rules PDF or a
// folder of downloaded archives, is never passed to citydb-tool.
func TestDiscoverDatasets_IgnoresEverythingButModelFolders(t *testing.T) {
	lod2Dir := t.TempDir()
	dir := filepath.Join(lod2Dir, "region-a")
	writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	if err := os.Remove(filepath.Join(dir, "gml")); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"tileset.json", "rules.pdf", "tile.gml"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "zips"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverDatasets(discoverConfig(lod2Dir, t.TempDir()))
	if err != nil {
		t.Fatalf("DiscoverDatasets: %v", err)
	}
	want := []importInput{{"cityjson", "CityJSON", filepath.Join(dir, "cityjson")}}
	if len(got) != 1 || !slices.Equal(got[0].Inputs, want) {
		t.Errorf("want only %+v as input, got %+v", want, got)
	}
}

func TestDiscoverDatasets_RejectsDatasetFolderWithoutModelFolder(t *testing.T) {
	lod2Dir := t.TempDir()
	dir := writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	for _, sub := range []string{"gml", "cityjson"} {
		if err := os.Remove(filepath.Join(dir, sub)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "zips"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverDatasets(discoverConfig(lod2Dir, t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Errorf("want an error naming %s, got %v", dir, err)
	}
}

func TestDiscoverDatasets_RejectsFolderWithoutValidAttribution(t *testing.T) {
	lod2Dir := t.TempDir()
	writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	bare := filepath.Join(lod2Dir, "region-b")
	if err := os.Mkdir(bare, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverDatasets(discoverConfig(lod2Dir, t.TempDir()))
	if err == nil || !strings.Contains(err.Error(), filepath.Join(bare, attribution.FileName)) {
		t.Errorf("want an error naming %s, got %v", filepath.Join(bare, attribution.FileName), err)
	}
}

func TestDiscoverDatasets_RejectsDuplicateDatasetID(t *testing.T) {
	lod2Dir, lod3Dir := t.TempDir(), t.TempDir()
	a := writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	b := writeDataset(t, lod3Dir, "region-a", "xx-region-a-lod2")

	_, err := DiscoverDatasets(discoverConfig(lod2Dir, lod3Dir))
	if err == nil || !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) {
		t.Errorf("want an error naming %s and %s, got %v", a, b, err)
	}
}

// An invalid attribution file in any folder must stop the run before the first
// citydb-tool import, so no data lands without a credit.
func TestImportCityDBData_InvalidAttributionImportsNothing(t *testing.T) {
	exeDir := t.TempDir()
	logPath := filepath.Join(exeDir, "invocations.log")
	writeFakeExecutable(t, exeDir, 0, logPath)
	lod2Dir := t.TempDir()
	writeDataset(t, lod2Dir, "region-a", "xx-region-a-lod2")
	broken := writeDataset(t, lod2Dir, "region-b", "xx-region-b-lod2")
	if err := os.WriteFile(filepath.Join(broken, attribution.FileName), []byte(`{"schema_version": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := cityDBDataConfig(t, exeDir, lod2Dir, t.TempDir())

	if err := ImportCityDBData(nil, cfg, "", ""); err == nil {
		t.Fatal("expected an error for the invalid attribution file, got nil")
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read invocation log: %v", err)
	}
	if strings.Contains(string(log), "import") {
		t.Errorf("expected no import before the attribution check, log:\n%s", log)
	}
}
