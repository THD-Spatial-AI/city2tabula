package importer

import (
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// A missing citydb binary must fail the run, not the process. This ran as
// log.Fatalf, so before the fix the whole test binary exited here and the
// package reported a failure with no assertion having run.
func TestImportCityDBData_MissingToolReturnsError(t *testing.T) {
	cfg := &config.Config{
		CityDB: &config.CityDB{ToolPath: t.TempDir()},
		Data:   &config.DataPaths{},
		DB:     &config.DBConfig{Schemas: &config.Schemas{}},
	}

	err := ImportCityDBData(nil, cfg, "", "")
	if err == nil {
		t.Fatal("expected an error for a missing citydb executable, got nil")
	}
	// The path is the thing an operator has to correct, so it has to be in the
	// message rather than only in the logs.
	if !strings.Contains(err.Error(), cfg.CityDB.ToolPath) {
		t.Errorf("error does not name the path that was wrong: %v", err)
	}
	if !strings.Contains(err.Error(), "CITYDB_TOOL_PATH") {
		t.Errorf("error does not name the variable to set: %v", err)
	}
}
