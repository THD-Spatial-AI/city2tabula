package db

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func TestParseSRID(t *testing.T) {
	cases := []struct {
		name    string
		crs     string
		want    int
		wantErr bool
	}{
		{"bare number", "25832", 25832, false},
		{"EPSG prefix", "EPSG:25832", 25832, false},
		{"lowercase epsg prefix", "epsg:25832", 25832, false},
		{"mixed case prefix", "Epsg:25832", 25832, false},
		{"surrounding whitespace", "  25832  ", 25832, false},
		{"whitespace with prefix", " EPSG:31256 ", 31256, false},
		{"non-numeric", "not-a-code", 0, true},
		{"empty string", "", 0, true},
		{"zero", "0", 0, true},
		{"negative", "-25832", 0, true},
		{"prefix with no number", "EPSG:", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSRID(tc.crs)

			if (err != nil) != tc.wantErr {
				t.Fatalf("parseSRID(%q) error = %v, wantErr %v", tc.crs, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("parseSRID(%q) = %d, want %d", tc.crs, got, tc.want)
			}
		})
	}
}

func TestExecuteCityDBScript_MissingScriptReturnsErrorWithoutTouchingDB(t *testing.T) {
	cfg := &config.Config{
		DB: &config.DBConfig{Host: "127.0.0.1", Port: "9999", Name: "nonexistent", User: "nobody", Password: "wrong"},
	}

	err := ExecuteCityDBScript(cfg, "/nonexistent/script/path.sql", "lod2")
	if err == nil {
		t.Fatal("expected an error for a missing script path, got nil")
	}
}
