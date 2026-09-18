package config

import "testing"

// baseWithPylovo is the shape LoadBaseConfig produces for the HTTP server.
func baseWithPylovo(fdw *PylovoFDW) Config {
	return Config{
		DB: &DBConfig{
			Name:    "city2tabula",
			Schemas: &Schemas{Pylvo: "some_other_schema"},
		},
		Data:      &DataPaths{},
		CityDB:    &CityDB{},
		PylovoFDW: fdw,
	}
}

// A server-triggered run has to reach PyLovo the same way the CLI does.
// RegionConfig dropped PylovoFDW, so Enabled() was always false on the server
// path: the FDW setup was skipped without error and no building_link rows were
// produced, which is the coverage the platform queries.
func TestRegionConfig_CarriesPylovoFDW(t *testing.T) {
	base := baseWithPylovo(&PylovoFDW{Host: "pylovo-db", Port: "5432", DBName: "pylovo"})

	cfg, err := RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}

	if !cfg.PylovoFDW.Enabled() {
		t.Fatal("PylovoFDW did not survive RegionConfig; a server-triggered run would silently skip PyLovo linking")
	}
	if cfg.PylovoFDW.Host != "pylovo-db" {
		t.Errorf("PylovoFDW.Host = %q, want pylovo-db", cfg.PylovoFDW.Host)
	}
	if cfg.DB.Schemas.Pylvo != PylvoFDWSchemaName {
		t.Errorf("Schemas.Pylvo = %q, want %q: in FDW mode the foreign tables live in the fixed schema",
			cfg.DB.Schemas.Pylvo, PylvoFDWSchemaName)
	}
}

// With FDW off, PYLOVO_SCHEMA still applies and must not be overwritten.
func TestRegionConfig_LeavesSchemaAloneWhenFDWDisabled(t *testing.T) {
	base := baseWithPylovo(&PylovoFDW{})

	cfg, err := RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	if cfg.PylovoFDW.Enabled() {
		t.Error("an empty host must not count as enabled")
	}
	if cfg.DB.Schemas.Pylvo != "some_other_schema" {
		t.Errorf("Schemas.Pylvo = %q, want it left as configured", cfg.DB.Schemas.Pylvo)
	}
}

// Each region gets its own Schemas copy, so switching one must not reach another.
func TestRegionConfig_SchemaSwitchDoesNotLeakToBase(t *testing.T) {
	base := baseWithPylovo(&PylovoFDW{Host: "pylovo-db"})

	if _, err := RegionConfig(base, "germany"); err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	if base.DB.Schemas.Pylvo != "some_other_schema" {
		t.Errorf("base Schemas.Pylvo was mutated to %q; regions share the base config",
			base.DB.Schemas.Pylvo)
	}
}
