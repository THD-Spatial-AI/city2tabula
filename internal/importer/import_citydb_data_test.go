package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// ImportCityDBData never uses its conn parameter, so nil is safe everywhere
// below - no real database needed for any of these, just fake stand-in
// "citydb" executables.

// writeHelpOnlyExecutable writes a fake "citydb" that succeeds on -help but
// fails on any "import" invocation - unlike writeFakeExecutable (single exit
// code for every call), this is needed to isolate the LOD2/LOD3 import
// failure branches from the earlier -help check, which must succeed first.
func writeHelpOnlyExecutable(t *testing.T, dir string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, "citydb")
	script := "#!/bin/sh\ncase \"$1\" in\n-help) exit 0 ;;\n*) exit 1 ;;\nesac\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake citydb executable: %v", err)
	}
	return scriptPath
}

func cityDBDataConfig(t *testing.T, toolDir, lod2Dir, lod3Dir string) *config.Config {
	t.Helper()
	cfg := minimalCityDBConfig()
	cfg.CityDB.ToolPath = toolDir
	cfg.DB.Schemas = &config.Schemas{Lod2: "lod2", Lod3: "lod3"}
	cfg.Data = &config.DataPaths{Lod2: lod2Dir, Lod3: lod3Dir}
	return cfg
}

func TestImportCityDBData_TestExecPathFailure(t *testing.T) {
	toolDir := t.TempDir()
	writeFakeExecutable(t, toolDir, 1, filepath.Join(toolDir, "log.txt")) // fails everything, including -help
	cfg := cityDBDataConfig(t, toolDir, "/nonexistent/lod2", "/nonexistent/lod3")

	err := ImportCityDBData(nil, cfg)
	if err == nil {
		t.Fatal("expected an error when the CityDB exec path test fails, got nil")
	}
}

func TestImportCityDBData_LOD2ImportFailure(t *testing.T) {
	toolDir := t.TempDir()
	writeHelpOnlyExecutable(t, toolDir)
	lod2Dir := t.TempDir() // exists -> importCityDBFiles won't skip it
	cfg := cityDBDataConfig(t, toolDir, lod2Dir, "/nonexistent/lod3")

	err := ImportCityDBData(nil, cfg)
	if err == nil {
		t.Fatal("expected an error when the LOD2 import fails, got nil")
	}
}

// TestImportCityDBData_LOD3ImportFailure isolates the LOD3 failure branch by
// letting LOD2 skip (nonexistent data dir, not a failure) so only LOD3's
// import is attempted.
func TestImportCityDBData_LOD3ImportFailure(t *testing.T) {
	toolDir := t.TempDir()
	writeHelpOnlyExecutable(t, toolDir)
	lod3Dir := t.TempDir()
	cfg := cityDBDataConfig(t, toolDir, "/nonexistent/lod2", lod3Dir)

	err := ImportCityDBData(nil, cfg)
	if err == nil {
		t.Fatal("expected an error when the LOD3 import fails, got nil")
	}
}
