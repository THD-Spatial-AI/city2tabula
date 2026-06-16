package importer

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func TestImportCsvWithPsql_ReturnsErrorOnFailure(t *testing.T) {
	// Given: a config pointing at a non-existent database
	cfg := &config.Config{
		DB: &config.DBConfig{
			Host:     "127.0.0.1",
			Port:     "9999",
			Name:     "nonexistent",
			User:     "nobody",
			Password: "wrong",
		},
	}

	// When: psql command is executed against an unreachable host
	err := ImportCsvWithPsql("/dev/null", cfg)

	// Then: error is propagated (psql exits non-zero)
	if err == nil {
		t.Error("expected error from psql against unreachable host, got nil")
	}
}
