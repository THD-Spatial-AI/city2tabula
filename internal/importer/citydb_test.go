package importer

import (
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

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
