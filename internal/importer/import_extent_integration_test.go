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
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\nexit 0\n", logPath)
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
