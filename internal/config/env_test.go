package config

import (
	"strings"
	"testing"
)

func TestGetEnv(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("CITY2TABULA_TEST_GETENV_SET", "value")
		if got := GetEnv("CITY2TABULA_TEST_GETENV_SET", "fallback"); got != "value" {
			t.Errorf("GetEnv() = %q, want %q", got, "value")
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		if got := GetEnv("CITY2TABULA_TEST_GETENV_UNSET", "fallback"); got != "fallback" {
			t.Errorf("GetEnv() = %q, want %q", got, "fallback")
		}
	})

	t.Run("returns fallback when set to empty string", func(t *testing.T) {
		t.Setenv("CITY2TABULA_TEST_GETENV_EMPTY", "")
		if got := GetEnv("CITY2TABULA_TEST_GETENV_EMPTY", "fallback"); got != "fallback" {
			t.Errorf("GetEnv() = %q, want %q", got, "fallback")
		}
	})
}

func TestGetEnvAsInt(t *testing.T) {
	t.Run("returns parsed int when valid", func(t *testing.T) {
		t.Setenv("CITY2TABULA_TEST_GETENVASINT_VALID", "42")
		if got := GetEnvAsInt("CITY2TABULA_TEST_GETENVASINT_VALID", 0); got != 42 {
			t.Errorf("GetEnvAsInt() = %d, want %d", got, 42)
		}
	})

	t.Run("returns fallback when unset", func(t *testing.T) {
		if got := GetEnvAsInt("CITY2TABULA_TEST_GETENVASINT_UNSET", 7); got != 7 {
			t.Errorf("GetEnvAsInt() = %d, want %d", got, 7)
		}
	})

	t.Run("returns fallback when not a valid int", func(t *testing.T) {
		t.Setenv("CITY2TABULA_TEST_GETENVASINT_INVALID", "not-a-number")
		if got := GetEnvAsInt("CITY2TABULA_TEST_GETENVASINT_INVALID", 7); got != 7 {
			t.Errorf("GetEnvAsInt() = %d, want %d", got, 7)
		}
	})
}

func TestNormalizeCountryName(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"already normalized", "germany", "germany"},
		{"mixed case", "Germany", "germany"},
		{"leading/trailing whitespace", "  germany  ", "germany"},
		{"internal spaces become underscores", "United Kingdom", "united_kingdom"},
		{"hyphens become underscores", "Czech-Republic", "czech_republic"},
		{"empty stays empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCountryName(tc.in); got != tc.want {
				t.Errorf("normalizeCountryName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestLoadEnv only confirms LoadEnv doesn't error out or panic when no .env
// file is present at the test's working directory (internal/config, not the
// project root) - godotenv.Overload's error is deliberately swallowed, so
// there is no observable branch to assert on beyond "it returns".
func TestLoadEnv(t *testing.T) {
	LoadEnv()
}

func validTestConfig() Config {
	return Config{
		Country:     "germany",
		CountryCode: "DE",
		DB: &DBConfig{
			Name:     "testdb",
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "secret",
		},
		CityDB: &CityDB{
			ToolPath: "/opt/citydb-tool",
			SRID:     "25832",
			SRSName:  "urn:ogc:def:crs:EPSG::25832",
		},
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config returns nil", func(t *testing.T) {
		if err := validTestConfig().Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("reports every missing field", func(t *testing.T) {
		cfg := validTestConfig()
		cfg.DB.Name = ""
		cfg.DB.Host = "  " // whitespace-only counts as missing
		cfg.DB.Port = ""
		cfg.DB.User = ""
		cfg.DB.Password = ""
		cfg.CityDB.ToolPath = ""
		cfg.CityDB.SRID = ""
		cfg.CityDB.SRSName = ""
		cfg.Country = ""

		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		for _, want := range []string{
			"DB_NAME", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD",
			"CITYDB_TOOL_PATH", "CITYDB_SRID", "CITYDB_SRS_NAME", "COUNTRY",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected error to mention %q, got: %v", want, err)
			}
		}
	})

	t.Run("unsupported country reports separately from missing fields", func(t *testing.T) {
		cfg := validTestConfig()
		cfg.CountryCode = ""

		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported country") {
			t.Errorf("expected an unsupported-country error, got: %v", err)
		}
	})
}
