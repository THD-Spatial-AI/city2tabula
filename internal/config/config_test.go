package config

import "testing"

func TestGetCountry(t *testing.T) {
	t.Setenv("COUNTRY", "  Germany  ")
	if got := getCountry(); got != "germany" {
		t.Errorf("getCountry() = %q, want %q", got, "germany")
	}
}

// TestLoadConfig exercises the whole assembly path, including the derived
// CountryCode lookup and every sub-loader it wires together.
func TestLoadConfig(t *testing.T) {
	t.Setenv("COUNTRY", "germany")
	t.Setenv("DB_HOST", "dbhost")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_NAME", "testdb")
	t.Setenv("DB_USER", "testuser")
	t.Setenv("DB_PASSWORD", "testpass")
	t.Setenv("CITYDB_TOOL_PATH", "/tools")
	t.Setenv("CITYDB_SRID", "25832")
	t.Setenv("CITYDB_SRS_NAME", "urn:ogc:def:crs:EPSG::25832")

	cfg := LoadConfig()

	if cfg.Country != "germany" {
		t.Errorf("Country = %q, want %q", cfg.Country, "germany")
	}
	if cfg.CountryCode != "DE" {
		t.Errorf("CountryCode = %q, want %q", cfg.CountryCode, "DE")
	}
	if cfg.DB == nil || cfg.DB.Host != "dbhost" {
		t.Errorf("DB = %+v, want Host=dbhost", cfg.DB)
	}
	if cfg.Data == nil || cfg.Data.Lod2 != Lod2DataDir+"germany" {
		t.Errorf("Data = %+v, want Lod2=%s", cfg.Data, Lod2DataDir+"germany")
	}
	if cfg.CityDB == nil || cfg.CityDB.ToolPath != "/tools" {
		t.Errorf("CityDB = %+v, want ToolPath=/tools", cfg.CityDB)
	}
	if cfg.City2Tabula == nil {
		t.Error("City2Tabula is nil")
	}
	if cfg.Batch == nil {
		t.Error("Batch is nil")
	}
	if cfg.RetryConfig == nil {
		t.Error("RetryConfig is nil")
	}
}

// TestLoadConfig_UnsupportedCountry covers LoadConfig's own handling of
// CountryCode's error return - it must still assemble a Config (with an
// empty CountryCode), leaving Validate() to catch it later rather than
// failing here.
func TestLoadConfig_UnsupportedCountry(t *testing.T) {
	t.Setenv("COUNTRY", "atlantis")

	cfg := LoadConfig()

	if cfg.Country != "atlantis" {
		t.Errorf("Country = %q, want %q", cfg.Country, "atlantis")
	}
	if cfg.CountryCode != "" {
		t.Errorf("CountryCode = %q, want empty for an unsupported country", cfg.CountryCode)
	}
}
