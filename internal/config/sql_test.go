package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestGetSQLParameters(t *testing.T) {
	cfg := &Config{
		Country:     "germany",
		CountryCode: "DE",
		DB: &DBConfig{
			Schemas: &Schemas{
				Lod2: "lod2", Lod3: "lod3", City2Tabula: "c2t", Tabula: "tab",
				Public: "public", CityDB: "citydb", CityDBPkg: "citydb_pkg", Pylvo: "pylovo",
			},
			Tables: &Tables{Tabula: "tab_table", TabulaVariant: "tab_variant_table"},
		},
		CityDB:      &CityDB{SRID: "25832"},
		City2Tabula: &City2TabulaConfig{RoomHeight: "2.5"},
	}

	cases := []struct {
		lod           int
		wantLodSchema string
	}{
		{2, "lod2"},
		{3, "lod3"},
		{0, ""}, // neither 2 nor 3 -> empty
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("lod=%d", tc.lod), func(t *testing.T) {
			params := cfg.GetSQLParameters(tc.lod, []int64{1, 2, 3})

			if params.LodSchema != tc.wantLodSchema {
				t.Errorf("LodSchema = %q, want %q", params.LodSchema, tc.wantLodSchema)
			}
			if params.LodLevel != tc.lod {
				t.Errorf("LodLevel = %d, want %d", params.LodLevel, tc.lod)
			}
			if len(params.BuildingIDs) != 3 {
				t.Errorf("BuildingIDs = %v, want 3 entries", params.BuildingIDs)
			}
			if params.SRID != "25832" {
				t.Errorf("SRID = %q, want %q", params.SRID, "25832")
			}
			if params.City2TabulaSchema != "c2t" {
				t.Errorf("City2TabulaSchema = %q, want %q", params.City2TabulaSchema, "c2t")
			}
			if params.TabulaSchema != "tab" {
				t.Errorf("TabulaSchema = %q, want %q", params.TabulaSchema, "tab")
			}
			if params.PublicSchema != "public" {
				t.Errorf("PublicSchema = %q, want %q", params.PublicSchema, "public")
			}
			if params.CityDBSchema != "citydb" {
				t.Errorf("CityDBSchema = %q, want %q", params.CityDBSchema, "citydb")
			}
			if params.CityDBPkgSchema != "citydb_pkg" {
				t.Errorf("CityDBPkgSchema = %q, want %q", params.CityDBPkgSchema, "citydb_pkg")
			}
			if params.Country != "germany" {
				t.Errorf("Country = %q, want %q", params.Country, "germany")
			}
			if params.CountryCode != "DE" {
				t.Errorf("CountryCode = %q, want %q", params.CountryCode, "DE")
			}
			if params.TabulaTable != "tab_table" {
				t.Errorf("TabulaTable = %q, want %q", params.TabulaTable, "tab_table")
			}
			if params.TabulaVariantTable != "tab_variant_table" {
				t.Errorf("TabulaVariantTable = %q, want %q", params.TabulaVariantTable, "tab_variant_table")
			}
			if params.RoomHeight != "2.5" {
				t.Errorf("RoomHeight = %q, want %q", params.RoomHeight, "2.5")
			}
			if params.PylvoSchema != "pylovo" {
				t.Errorf("PylvoSchema = %q, want %q", params.PylvoSchema, "pylovo")
			}
		})
	}
}

func TestLoadSQLFilesFromDir(t *testing.T) {
	t.Run("sorts files and ignores non-.sql files", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"02_second.sql", "01_first.sql", "10_tenth.sql", "readme.txt"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0644); err != nil {
				t.Fatalf("failed to write fixture %s: %v", name, err)
			}
		}

		got, err := loadSQLFilesFromDir(dir)
		if err != nil {
			t.Fatalf("loadSQLFilesFromDir: %v", err)
		}

		want := []string{
			filepath.Join(dir, "01_first.sql"),
			filepath.Join(dir, "02_second.sql"),
			filepath.Join(dir, "10_tenth.sql"),
		}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("empty directory returns an error", func(t *testing.T) {
		_, err := loadSQLFilesFromDir(t.TempDir())
		if err == nil {
			t.Fatal("expected an error for a directory with no .sql files")
		}
		if !strings.Contains(err.Error(), "no SQL files found") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("nonexistent directory returns an error", func(t *testing.T) {
		// filepath.Glob doesn't error on a missing directory, it just finds
		// no matches - so this hits the same "no SQL files found" branch.
		_, err := loadSQLFilesFromDir(filepath.Join(t.TempDir(), "does", "not", "exist"))
		if err == nil {
			t.Fatal("expected an error for a nonexistent directory")
		}
		if !strings.Contains(err.Error(), "no SQL files found") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("malformed glob pattern returns a glob error", func(t *testing.T) {
		// An unmatched "[" in the directory name makes filepath.Glob itself
		// fail (ErrBadPattern) rather than simply finding no matches.
		_, err := loadSQLFilesFromDir(filepath.Join(t.TempDir(), "bad[pattern"))
		if err == nil {
			t.Fatal("expected a glob syntax error")
		}
		if strings.Contains(err.Error(), "no SQL files found") {
			t.Errorf("expected a glob syntax error, not the empty-dir error: %v", err)
		}
	})
}

// chdirWithSQLTree creates a fresh temp dir, chdirs the test into it, and
// creates only the given relative directories (each with one stub .sql
// file) - letting a test control exactly which of LoadSQLScripts' six
// sequential loads succeeds and which one is first to fail.
func chdirWithSQLTree(t *testing.T, dirs []string) {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)

	for _, d := range dirs {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", d, err)
		}
		if err := os.WriteFile(filepath.Join(full, "01_stub.sql"), []byte("SELECT 1;"), 0644); err != nil {
			t.Fatalf("failed to write stub SQL into %s: %v", d, err)
		}
	}
}

func TestConfig_LoadSQLScripts_Failures(t *testing.T) {
	// In LoadSQLScripts' call order - each case creates every directory up
	// to (but not including) the one that should be missing, isolating that
	// specific early-return branch.
	allDirs := []string{
		strings.TrimSuffix(SQLMainScriptDir, string(os.PathSeparator)),
		strings.TrimSuffix(SQLSupplementaryScriptDir, string(os.PathSeparator)),
		strings.TrimSuffix(SQLPylvoLinkScriptDir, string(os.PathSeparator)),
		strings.TrimSuffix(SQLMainSchemaPath, string(os.PathSeparator)),
		strings.TrimSuffix(SQLSupplementarySchemaPath, string(os.PathSeparator)),
		strings.TrimSuffix(SQLTrainingFunctionsPath, string(os.PathSeparator)),
	}

	cases := []struct {
		name       string
		createUpTo int
	}{
		{"main scripts missing", 0},
		{"supplementary scripts missing", 1},
		{"pylovo link scripts missing", 2},
		{"main schema missing", 3},
		{"supplementary schema missing", 4},
		{"function scripts missing", 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirWithSQLTree(t, allDirs[:tc.createUpTo])

			cfg := &Config{}
			_, err := cfg.LoadSQLScripts()
			if err == nil {
				t.Fatalf("expected an error when %s", tc.name)
			}
		})
	}
}

func TestConfig_LoadSQLScripts_Success(t *testing.T) {
	t.Chdir(projectRoot())

	cfg := &Config{}
	scripts, err := cfg.LoadSQLScripts()
	if err != nil {
		t.Fatalf("LoadSQLScripts: %v", err)
	}

	for name, got := range map[string][]string{
		"MainScripts":               scripts.MainScripts,
		"SupplementaryScripts":      scripts.SupplementaryScripts,
		"PyLovoLinkScripts":         scripts.PyLovoLinkScripts,
		"MainTableScripts":          scripts.MainTableScripts,
		"SupplementaryTableScripts": scripts.SupplementaryTableScripts,
		"FunctionScripts":           scripts.FunctionScripts,
	} {
		if len(got) == 0 {
			t.Errorf("%s: expected at least one script, got none", name)
		}
	}
}
