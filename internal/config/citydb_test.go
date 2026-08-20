package config

import (
	"path/filepath"
	"testing"
)

func TestLoadCityDBConfig_Defaults(t *testing.T) {
	// Given: no environment variables set
	t.Setenv("CITYDB_TOOL_PATH", "")
	t.Setenv("CITYDB_SRS_NAME", "")
	t.Setenv("CITYDB_SRID", "")
	t.Setenv("IMPORT_LIMIT", "")

	// When
	cfg := loadCityDBConfig("")

	// Then
	if cfg.ToolPath != "" {
		t.Errorf("ToolPath: got %q, want empty", cfg.ToolPath)
	}
	if cfg.ImportLimit != 0 {
		t.Errorf("ImportLimit: got %d, want 0", cfg.ImportLimit)
	}
	if len(cfg.LODLevels) != 2 || cfg.LODLevels[0] != 2 || cfg.LODLevels[1] != 3 {
		t.Errorf("LODLevels: got %v, want [2 3]", cfg.LODLevels)
	}
}

func TestLoadCityDBConfig_DerivesSRIDFromCountry(t *testing.T) {
	t.Setenv("CITYDB_SRS_NAME", "")
	t.Setenv("CITYDB_SRID", "")

	cfg := loadCityDBConfig("germany")

	if cfg.SRID != "25832" {
		t.Errorf("SRID: got %q, want %q (derived from country)", cfg.SRID, "25832")
	}
	if cfg.SRSName != "ETRS89 / UTM zone 32N" {
		t.Errorf("SRSName: got %q, want the derived value", cfg.SRSName)
	}
}

func TestLoadCityDBConfig_ExplicitSRIDOverridesDerivation(t *testing.T) {
	t.Setenv("CITYDB_SRID", "9999")
	t.Setenv("CITYDB_SRS_NAME", "Custom CRS")

	cfg := loadCityDBConfig("germany")

	if cfg.SRID != "9999" {
		t.Errorf("SRID: got %q, want the explicit override %q", cfg.SRID, "9999")
	}
	if cfg.SRSName != "Custom CRS" {
		t.Errorf("SRSName: got %q, want the explicit override", cfg.SRSName)
	}
}

func TestLoadCityDBConfig_UnsupportedCountryLeavesSRIDEmpty(t *testing.T) {
	t.Setenv("CITYDB_SRS_NAME", "")
	t.Setenv("CITYDB_SRID", "")

	cfg := loadCityDBConfig("narnia")

	if cfg.SRID != "" || cfg.SRSName != "" {
		t.Errorf("SRID/SRSName: got %q/%q, want both empty for an unsupported country", cfg.SRID, cfg.SRSName)
	}
}

func TestLoadCityDBConfig_SQLScriptPaths(t *testing.T) {
	// Given
	t.Setenv("CITYDB_TOOL_PATH", "/opt/citydb")

	// When
	cfg := loadCityDBConfig("")

	// Then: script paths are derived from CITYDB_TOOL_PATH
	base := filepath.Join("/opt/citydb", "3dcitydb", "postgresql", "sql-scripts")
	cases := []struct {
		got  string
		name string
		file string
	}{
		{cfg.SQLScripts.CreateDB, "CreateDB", "create-db.sql"},
		{cfg.SQLScripts.CreateSchema, "CreateSchema", "create-schema.sql"},
		{cfg.SQLScripts.DropDB, "DropDB", "drop-db.sql"},
		{cfg.SQLScripts.DropSchema, "DropSchema", "drop-schema.sql"},
	}
	for _, tc := range cases {
		want := filepath.Join(base, tc.file)
		if tc.got != want {
			t.Errorf("SQLScripts.%s: got %q, want %q", tc.name, tc.got, want)
		}
	}
}

func TestLoadCityDBConfig_ImportLimit(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   int
	}{
		{"valid positive", "50", 50},
		{"zero", "0", 0},
		{"negative is rejected", "-1", 0},
		{"invalid string is rejected", "abc", 0},
		{"empty string uses default", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			t.Setenv("IMPORT_LIMIT", tc.envVal)

			// When
			cfg := loadCityDBConfig("")

			// Then
			if cfg.ImportLimit != tc.want {
				t.Errorf("ImportLimit: got %d, want %d", cfg.ImportLimit, tc.want)
			}
		})
	}
}
