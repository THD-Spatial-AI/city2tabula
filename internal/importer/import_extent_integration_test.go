//go:build integration

package importer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
)

// extentConfig gives a test its own LOD2 schema, one LOD2 dataset holding a
// single model file, and a fake citydb that appends each invocation to a log.
// It returns the config, the log path and the dataset's gml folder.
func extentConfig(t *testing.T, ctx context.Context, schema string) (*config.Config, string, string) {
	t.Helper()
	if _, err := testPool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })

	toolDir := t.TempDir()
	logPath := filepath.Join(toolDir, "calls.txt")
	// An argument file (@path) is logged as a FILES line, since the importer
	// removes it once citydb-tool returns.
	script := fmt.Sprintf("#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in @*) printf 'FILES %%s\\n' \"$(tr '\\n' ' ' < \"${a#@}\")\" >> %[1]q;; esac; done\necho \"$@\" >> %[1]q\nexit 0\n", logPath)
	if err := os.WriteFile(filepath.Join(toolDir, "citydb"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake citydb: %v", err)
	}

	lod2 := t.TempDir()
	dataset := filepath.Join(lod2, "region-a")
	writeAttributionFolder(t, dataset, "xx-region-a-lod2", "Credit A")
	gml := filepath.Join(dataset, "gml")
	if err := os.WriteFile(filepath.Join(gml, "tile-1.gml"), []byte("<tile/>"), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}

	cfg := testConfig()
	cfg.CityDB.ToolPath = toolDir
	cfg.DB.Schemas.Lod2 = schema
	cfg.Data = &config.DataPaths{Lod2: lod2, Lod3: filepath.Join(lod2, "missing-lod3")}
	return cfg, logPath, gml
}

// importCalls counts the import invocations the fake citydb has logged.
func importCalls(t *testing.T, logPath string) int {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read citydb call log %s: %v", logPath, err)
	}
	return strings.Count(string(data), "import ")
}

func TestImportCityDBData_SkipsBboxAlreadyImported(t *testing.T) {
	ctx := context.Background()
	cfg, logPath, _ := extentConfig(t, ctx, "extent_skip_lod2")

	steps := []struct {
		name, bbox, mode string
		wantCalls        int
	}{
		{"first import", "16.349,48.210,16.351,48.212,4326", "intersects", 1},
		{"same bbox", "16.349,48.210,16.351,48.212,4326", "intersects", 1},
		{"bbox inside", "16.3495,48.2105,16.3505,48.2115,4326", "intersects", 1},
		{"contains mode inside an intersects import", "16.3495,48.2105,16.3505,48.2115,4326", "contains", 1},
		{"bbox reaching outside", "16.348,48.210,16.351,48.212,4326", "intersects", 2},
	}
	for _, s := range steps {
		if err := importer.ImportCityDBData(testPool, cfg, s.bbox, s.mode); err != nil {
			t.Fatalf("%s: ImportCityDBData: %v", s.name, err)
		}
		if got := importCalls(t, logPath); got != s.wantCalls {
			t.Fatalf("%s: citydb import called %d times in total, want %d", s.name, got, s.wantCalls)
		}
	}
}

func TestImportCityDBData_ReimportsWhenSourceFilesChange(t *testing.T) {
	ctx := context.Background()
	cfg, logPath, gml := extentConfig(t, ctx, "extent_change_lod2")
	bbox := "16.349,48.210,16.351,48.212,4326"

	if err := importer.ImportCityDBData(testPool, cfg, bbox, "intersects"); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gml, "tile-2.gml"), []byte("<tile/>"), 0o644); err != nil {
		t.Fatalf("add model file: %v", err)
	}
	if err := importer.ImportCityDBData(testPool, cfg, bbox, "intersects"); err != nil {
		t.Fatalf("import after adding a file: %v", err)
	}
	if got := importCalls(t, logPath); got != 2 {
		t.Fatalf("citydb import called %d times, want 2 (a new source file must be imported)", got)
	}
}

func TestImportCityDBData_ImportLimitIsNotRecorded(t *testing.T) {
	ctx := context.Background()
	cfg, logPath, _ := extentConfig(t, ctx, "extent_limit_lod2")
	cfg.CityDB.ImportLimit = 10
	bbox := "16.349,48.210,16.351,48.212,4326"

	for i := 0; i < 2; i++ {
		if err := importer.ImportCityDBData(testPool, cfg, bbox, "intersects"); err != nil {
			t.Fatalf("import %d: %v", i+1, err)
		}
	}
	if got := importCalls(t, logPath); got != 2 {
		t.Fatalf("citydb import called %d times, want 2 (a limited import is partial)", got)
	}
}

// TestImportCityDBData_SkipsUnchangedWholeFolder pins that a whole-folder import
// (no bbox), as a scheduled run makes, is recorded: repeating it, or a bbox
// import afterwards, skips citydb-tool until the folder's files change.
func TestImportCityDBData_SkipsUnchangedWholeFolder(t *testing.T) {
	ctx := context.Background()
	cfg, logPath, gml := extentConfig(t, ctx, "extent_whole_lod2")

	steps := []struct {
		name, bbox, mode string
		addFile          string
		wantCalls        int
	}{
		{name: "first whole-folder import", wantCalls: 1},
		{name: "same files again", wantCalls: 1},
		{name: "bbox import after a whole-folder one", bbox: "16.349,48.210,16.351,48.212,4326", mode: "intersects", wantCalls: 1},
		{name: "a file added", addFile: "tile-2.gml", wantCalls: 2},
		{name: "same files again after the change", wantCalls: 2},
	}
	for _, s := range steps {
		if s.addFile != "" {
			if err := os.WriteFile(filepath.Join(gml, s.addFile), []byte("<tile/>"), 0o644); err != nil {
				t.Fatalf("%s: %v", s.name, err)
			}
		}
		if err := importer.ImportCityDBData(testPool, cfg, s.bbox, s.mode); err != nil {
			t.Fatalf("%s: ImportCityDBData: %v", s.name, err)
		}
		if got := importCalls(t, logPath); got != s.wantCalls {
			t.Fatalf("%s: citydb import called %d times in total, want %d", s.name, got, s.wantCalls)
		}
	}
}

// tileGML is a CityGML file whose envelope covers x0..x0+2000, y0..y0+2000 in
// EPSG:25832.
func tileGML(x0, y0 float64) string {
	return fmt.Sprintf(`<core:CityModel><gml:boundedBy><gml:Envelope srsName="urn:ogc:def:crs:EPSG::25832">`+
		`<gml:lowerCorner>%[1]f %[2]f 0</gml:lowerCorner><gml:upperCorner>%[3]f %[4]f 100</gml:upperCorner>`+
		`</gml:Envelope></gml:boundedBy></core:CityModel>`, x0, y0, x0+2000, y0+2000)
}

// bboxIn4326 returns an import bbox for an EPSG:25832 rectangle.
func bboxIn4326(t *testing.T, ctx context.Context, xmin, ymin, xmax, ymax float64) string {
	t.Helper()
	var b [4]float64
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, 25832), 4326) AS e) s`,
		xmin, ymin, xmax, ymax).Scan(&b[0], &b[1], &b[2], &b[3]); err != nil {
		t.Fatalf("transform bbox: %v", err)
	}
	return fmt.Sprintf("%f,%f,%f,%f,4326", b[0], b[1], b[2], b[3])
}

// lastFiles returns the file names of the last argument file citydb was given.
func lastFiles(t *testing.T, logPath string) string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read citydb call log %s: %v", logPath, err)
	}
	last := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "FILES ") {
			last = ""
			for _, f := range strings.Fields(strings.TrimPrefix(line, "FILES ")) {
				last += filepath.Base(strings.Trim(f, `"`)) + " "
			}
		}
	}
	return strings.TrimSpace(last)
}

// TestImportCityDBData_SelectsFilesInTheBbox pins that a bbox import passes
// citydb-tool only the files whose declared extent intersects the bbox, and
// records the bbox against those files alone: adding a file elsewhere leaves the
// record valid, and a bbox no file reaches runs no import.
func TestImportCityDBData_SelectsFilesInTheBbox(t *testing.T) {
	ctx := context.Background()
	cfg, logPath, gml := extentConfig(t, ctx, "extent_select_lod2")
	if err := os.Remove(filepath.Join(gml, "tile-1.gml")); err != nil {
		t.Fatal(err)
	}
	write := func(name string, x0, y0 float64) {
		if err := os.WriteFile(filepath.Join(gml, name), []byte(tileGML(x0, y0)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.gml", 786000, 5416000)
	write("b.gml", 790000, 5416000)
	regionA := bboxIn4326(t, ctx, 786500, 5416500, 787500, 5417500)
	regionB := bboxIn4326(t, ctx, 790500, 5416500, 793500, 5417500)
	elsewhere := bboxIn4326(t, ctx, 800500, 5416500, 801500, 5417500)

	steps := []struct {
		name, bbox, add string
		wantCalls       int
		wantFiles       string
	}{
		{name: "region A", bbox: regionA, wantCalls: 1, wantFiles: "a.gml"},
		{name: "region A again", bbox: regionA, wantCalls: 1, wantFiles: "a.gml"},
		{name: "region A after a tile is added in region B", bbox: regionA, add: "c.gml", wantCalls: 1, wantFiles: "a.gml"},
		{name: "region B", bbox: regionB, wantCalls: 2, wantFiles: "b.gml c.gml"},
		{name: "a bbox no file reaches", bbox: elsewhere, wantCalls: 2, wantFiles: "b.gml c.gml"},
	}
	for _, s := range steps {
		if s.add != "" {
			write(s.add, 792000, 5416000)
		}
		if err := importer.ImportCityDBData(testPool, cfg, s.bbox, "intersects"); err != nil {
			t.Fatalf("%s: ImportCityDBData: %v", s.name, err)
		}
		if got := importCalls(t, logPath); got != s.wantCalls {
			t.Fatalf("%s: citydb import called %d times in total, want %d", s.name, got, s.wantCalls)
		}
		if got := lastFiles(t, logPath); got != s.wantFiles {
			t.Fatalf("%s: last import got files %q, want %q", s.name, got, s.wantFiles)
		}
	}
}
