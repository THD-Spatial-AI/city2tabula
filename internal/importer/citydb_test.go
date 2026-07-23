package importer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// writeFakeExecutable writes an executable shell script named "citydb" to dir that
// appends its arguments to logPath (one line per invocation) and exits with
// exitCode. Standing in for the real CityDB Java CLI tool, which isn't available
// in the test environment - real process, fake binary, so testCityDBExecPath /
// executeCityDBCommand / importCityDBFiles / ImportCityDBData can be exercised
// without mocking exec.Command itself.
func writeFakeExecutable(t *testing.T, dir string, exitCode int, logPath string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, "citydb")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\nexit %d\n", logPath, exitCode)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake citydb executable: %v", err)
	}
	return scriptPath
}

func minimalCityDBConfig() *config.Config {
	return &config.Config{
		DB: &config.DBConfig{
			Name:     "testdb",
			User:     "user",
			Password: "pass",
			Host:     "localhost",
			Port:     "5432",
		},
		Batch: &config.BatchConfig{
			Threads: 4,
		},
		CityDB: &config.CityDB{
			ImportLimit: 0,
		},
	}
}

func TestGetCityDBImportCommand_ArgsStructure(t *testing.T) {
	// Given
	cfg := minimalCityDBConfig()

	// When
	cmd := getCityDBImportCommand("/bin/citydb", "/data/lod2", "lod2", "citygml", cfg)

	// Then: data path is the final argument
	args := cmd.Args
	if args[len(args)-1] != "/data/lod2" {
		t.Errorf("last arg: got %q, want /data/lod2", args[len(args)-1])
	}

	// Then: no --limit flag when ImportLimit is 0
	for _, a := range args {
		if strings.HasPrefix(a, "--limit=") {
			t.Errorf("unexpected --limit flag when ImportLimit=0, args: %v", args)
		}
	}
}

func TestGetCityDBImportCommand_WithImportLimit(t *testing.T) {
	// Given
	cfg := minimalCityDBConfig()
	cfg.CityDB.ImportLimit = 10

	// When
	cmd := getCityDBImportCommand("/bin/citydb", "/data/lod2", "lod2", "citygml", cfg)

	// Then: --limit flag is present
	found := false
	for _, a := range cmd.Args {
		if a == "--limit=10" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --limit=10 in args, got %v", cmd.Args)
	}
}

func TestGetCityDBImportCommand_FormatAndSchema(t *testing.T) {
	// Given
	cfg := minimalCityDBConfig()

	cases := []struct {
		format string
		schema string
	}{
		{"citygml", "lod2"},
		{"cityjson", "lod3"},
	}

	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			// When
			cmd := getCityDBImportCommand("/bin/citydb", "/data", tc.schema, tc.format, cfg)

			// Then: format and schema appear in args
			foundFormat, foundSchema := false, false
			for _, a := range cmd.Args {
				if a == tc.format {
					foundFormat = true
				}
				if a == "--db-schema="+tc.schema {
					foundSchema = true
				}
			}
			if !foundFormat {
				t.Errorf("format %q not found in args %v", tc.format, cmd.Args)
			}
			if !foundSchema {
				t.Errorf("schema flag --db-schema=%s not found in args %v", tc.schema, cmd.Args)
			}
		})
	}
}

func TestImportCityDBFiles_MissingPathReturnsNil(t *testing.T) {
	// Given: a data path that does not exist
	cfg := minimalCityDBConfig()

	// When
	err := importCityDBFiles("/bin/citydb", "/nonexistent/path/xyz", "lod2", "LOD2", cfg)

	// Then: missing path is treated as optional — skip with no error
	if err != nil {
		t.Errorf("expected nil error for missing data path, got %v", err)
	}
}

func TestTestCityDBExecPath_Success(t *testing.T) {
	dir := t.TempDir()
	exe := writeFakeExecutable(t, dir, 0, filepath.Join(dir, "log.txt"))

	if err := testCityDBExecPath(exe); err != nil {
		t.Errorf("expected nil error when -help succeeds, got %v", err)
	}
}

func TestTestCityDBExecPath_Failure(t *testing.T) {
	dir := t.TempDir()
	exe := writeFakeExecutable(t, dir, 1, filepath.Join(dir, "log.txt"))

	if err := testCityDBExecPath(exe); err == nil {
		t.Error("expected an error when -help fails, got nil")
	}
}

func TestExecuteCityDBCommand_Success(t *testing.T) {
	dir := t.TempDir()
	exe := writeFakeExecutable(t, dir, 0, filepath.Join(dir, "log.txt"))
	cmd := exec.Command(exe, "import")

	if err := executeCityDBCommand(cmd, "test import"); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestExecuteCityDBCommand_Failure(t *testing.T) {
	dir := t.TempDir()
	exe := writeFakeExecutable(t, dir, 1, filepath.Join(dir, "log.txt"))
	cmd := exec.Command(exe, "import")

	if err := executeCityDBCommand(cmd, "test import"); err == nil {
		t.Error("expected an error when the command exits non-zero, got nil")
	}
}

func TestImportCityDBFiles_BothFormatsAttemptedOnSuccess(t *testing.T) {
	dataDir := t.TempDir() // exists -> passes the os.Stat check
	exeDir := t.TempDir()
	logPath := filepath.Join(exeDir, "invocations.log")
	exe := writeFakeExecutable(t, exeDir, 0, logPath)
	cfg := minimalCityDBConfig()

	if err := importCityDBFiles(exe, dataDir, "lod2", "LOD2", cfg); err != nil {
		t.Fatalf("importCityDBFiles: %v", err)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}
	for _, want := range []string{"citygml", "cityjson"} {
		if !strings.Contains(string(logContent), want) {
			t.Errorf("expected %q to have been invoked, log:\n%s", want, logContent)
		}
	}
}

// TestImportCityDBFiles_CityGMLFailureShortCircuitsCityJSON guards the early
// return in importCityDBFiles's format loop: if the CityGML import fails,
// CityJSON must never be attempted for that LOD level.
func TestImportCityDBFiles_CityGMLFailureShortCircuitsCityJSON(t *testing.T) {
	dataDir := t.TempDir()
	exeDir := t.TempDir()
	logPath := filepath.Join(exeDir, "invocations.log")
	exe := writeFakeExecutable(t, exeDir, 1, logPath) // always fails
	cfg := minimalCityDBConfig()

	if err := importCityDBFiles(exe, dataDir, "lod2", "LOD2", cfg); err == nil {
		t.Fatal("expected an error when the citygml import fails, got nil")
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 invocation (citygml only, cityjson short-circuited), got %d:\n%s", len(lines), logContent)
	}
	if !strings.Contains(lines[0], "citygml") {
		t.Errorf("expected the one invocation to be citygml, got: %s", lines[0])
	}
}

// TestImportCityDBData_Success drives ImportCityDBData end to end (exec-path
// check, -help test, LOD2 + LOD3 imports) against a fake executable. The one
// branch deliberately left untouched is the missing-exec-path case, which calls
// utils.Error.Fatalf (os.Exit) and can't be exercised without killing the test
// process.
func TestImportCityDBData_Success(t *testing.T) {
	exeDir := t.TempDir()
	logPath := filepath.Join(exeDir, "invocations.log")
	writeFakeExecutable(t, exeDir, 0, logPath) // must be named "citydb" inside exeDir

	lod2Dir, lod3Dir := t.TempDir(), t.TempDir()
	cfg := minimalCityDBConfig()
	cfg.CityDB.ToolPath = exeDir
	cfg.Data = &config.DataPaths{Lod2: lod2Dir, Lod3: lod3Dir}
	cfg.DB.Schemas = &config.Schemas{Lod2: "lod2", Lod3: "lod3"}

	if err := ImportCityDBData(nil, cfg); err != nil {
		t.Fatalf("ImportCityDBData: %v", err)
	}

	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read invocation log: %v", err)
	}
	// -help, then citygml+cityjson for each of LOD2 and LOD3 = 5 invocations.
	lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 invocations (-help + 2 formats x 2 LOD levels), got %d:\n%s", len(lines), logContent)
	}
}
