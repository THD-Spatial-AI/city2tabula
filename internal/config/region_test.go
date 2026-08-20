package config

import "testing"

func baseTestConfig(t *testing.T) Config {
	t.Helper()
	t.Setenv("DB_NAME", "city2tabula")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("CITYDB_TOOL_PATH", "/opt/citydb-tool")
	t.Setenv("COUNTRY", "") // base config must not require COUNTRY
	return LoadBaseConfig()
}

func TestLoadBaseConfig_NoCountryRequired(t *testing.T) {
	base := baseTestConfig(t)

	if base.Country != "" {
		t.Errorf("Country = %q, want empty (base config is country-independent)", base.Country)
	}
	if base.DB.Name != "city2tabula" {
		t.Errorf("DB.Name = %q, want unsuffixed base name %q", base.DB.Name, "city2tabula")
	}
}

func TestRegionConfig_DerivesPerCountryFields(t *testing.T) {
	base := baseTestConfig(t)

	cfg, err := RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}

	if cfg.Country != "germany" {
		t.Errorf("Country = %q, want %q", cfg.Country, "germany")
	}
	if cfg.CountryCode != "DE" {
		t.Errorf("CountryCode = %q, want %q", cfg.CountryCode, "DE")
	}
	if cfg.DB.Name != "city2tabula_de" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "city2tabula_de")
	}
	if cfg.CityDB.SRID != "25832" {
		t.Errorf("CityDB.SRID = %q, want %q", cfg.CityDB.SRID, "25832")
	}
	if cfg.Data.Lod2 != Lod2DataDir+"germany" {
		t.Errorf("Data.Lod2 = %q, want %q", cfg.Data.Lod2, Lod2DataDir+"germany")
	}
}

func TestRegionConfig_IndependentPerCall(t *testing.T) {
	base := baseTestConfig(t)

	de, err := RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig(germany): %v", err)
	}
	at, err := RegionConfig(base, "austria")
	if err != nil {
		t.Fatalf("RegionConfig(austria): %v", err)
	}

	if de.DB.Name == at.DB.Name {
		t.Errorf("expected different DB names for germany/austria, both got %q", de.DB.Name)
	}
	if de.DB == at.DB {
		t.Error("expected independent *DBConfig pointers per call, got the same pointer")
	}
	// base itself must stay unmodified by either call.
	if base.DB.Name != "city2tabula" {
		t.Errorf("base.DB.Name mutated to %q, want unchanged %q", base.DB.Name, "city2tabula")
	}
}

func TestRegionConfig_UnsupportedCountry(t *testing.T) {
	base := baseTestConfig(t)

	if _, err := RegionConfig(base, "atlantis"); err == nil {
		t.Error("RegionConfig(\"atlantis\") expected error, got nil")
	}
}
