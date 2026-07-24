package config

import "testing"

func TestLoadDBConfig(t *testing.T) {
	t.Setenv("DB_HOST", "myhost")
	t.Setenv("DB_PORT", "1234")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_USER", "myuser")
	t.Setenv("DB_PASSWORD", "mypass")
	t.Setenv("DB_SSL_MODE", "require")

	db := loadDBConfig()

	if db.Host != "myhost" {
		t.Errorf("Host = %q, want %q", db.Host, "myhost")
	}
	if db.Port != "1234" {
		t.Errorf("Port = %q, want %q", db.Port, "1234")
	}
	if db.Name != "mydb" {
		t.Errorf("Name = %q, want %q", db.Name, "mydb")
	}
	if db.User != "myuser" {
		t.Errorf("User = %q, want %q", db.User, "myuser")
	}
	if db.Password != "mypass" {
		t.Errorf("Password = %q, want %q", db.Password, "mypass")
	}
	if db.SSLMode != "require" {
		t.Errorf("SSLMode = %q, want %q", db.SSLMode, "require")
	}
	if db.Tables == nil {
		t.Error("Tables is nil")
	}
	if db.Schemas == nil {
		t.Error("Schemas is nil")
	}
	if db.SQL != nil {
		t.Error("SQL should be nil until LoadSQLScripts populates it")
	}
}

func TestLoadDBConfig_Defaults(t *testing.T) {
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD", "DB_SSL_MODE"} {
		t.Setenv(key, "")
	}

	db := loadDBConfig()

	if db.Host != "localhost" {
		t.Errorf("Host = %q, want default %q", db.Host, "localhost")
	}
	if db.Port != "5432" {
		t.Errorf("Port = %q, want default %q", db.Port, "5432")
	}
	if db.User != "postgres" {
		t.Errorf("User = %q, want default %q", db.User, "postgres")
	}
	if db.Name != "" {
		t.Errorf("Name = %q, want default empty", db.Name)
	}
}

func TestLoadSchemas(t *testing.T) {
	t.Setenv("PYLOVO_SCHEMA", "custom_pylovo")

	s := loadSchemas()

	cases := map[string]struct{ got, want string }{
		"Public":      {s.Public, PublicSchema},
		"CityDB":      {s.CityDB, CityDBSchema},
		"CityDBPkg":   {s.CityDBPkg, CityDBPkgSchema},
		"Lod2":        {s.Lod2, Lod2Schema},
		"Lod3":        {s.Lod3, Lod3Schema},
		"Tabula":      {s.Tabula, TabulaSchema},
		"City2Tabula": {s.City2Tabula, City2TabulaSchema},
		"Pylvo":       {s.Pylvo, "custom_pylovo"},
	}
	for field, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", field, c.got, c.want)
		}
	}
}

func TestLoadSchemas_PylvoDefault(t *testing.T) {
	t.Setenv("PYLOVO_SCHEMA", "")

	s := loadSchemas()

	if s.Pylvo != PylvoSchemaName {
		t.Errorf("Pylvo = %q, want default %q", s.Pylvo, PylvoSchemaName)
	}
}

func TestSchemas_All(t *testing.T) {
	s := loadSchemas()

	got := s.All()
	want := []string{s.Public, s.CityDB, s.CityDBPkg, s.Lod2, s.Lod3, s.Tabula, s.City2Tabula}

	if len(got) != len(want) {
		t.Fatalf("All() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadTables(t *testing.T) {
	tb := loadTables()

	if tb.Tabula != Tabula {
		t.Errorf("Tabula = %q, want %q", tb.Tabula, Tabula)
	}
	if tb.TabulaVariant != TabulaVariant {
		t.Errorf("TabulaVariant = %q, want %q", tb.TabulaVariant, TabulaVariant)
	}
}
