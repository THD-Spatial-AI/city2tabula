package config

import (
	"os"
	"testing"
)

func TestLoadDataPaths(t *testing.T) {
	t.Setenv("COUNTRY", "Germany")

	dp := loadDataPaths()

	if dp.Base != DataDir {
		t.Errorf("Base = %q, want %q", dp.Base, DataDir)
	}
	if want := Lod2DataDir + "germany"; dp.Lod2 != want {
		t.Errorf("Lod2 = %q, want %q", dp.Lod2, want)
	}
	if want := Lod3DataDir + "germany"; dp.Lod3 != want {
		t.Errorf("Lod3 = %q, want %q", dp.Lod3, want)
	}
	if dp.Tabula != TabulaDataDir {
		t.Errorf("Tabula = %q, want %q", dp.Tabula, TabulaDataDir)
	}
}

func TestLoadDataPaths_UnsetCountry(t *testing.T) {
	t.Setenv("COUNTRY", "")

	dp := loadDataPaths()

	if dp.Lod2 != Lod2DataDir {
		t.Errorf("Lod2 = %q, want %q (no country suffix)", dp.Lod2, Lod2DataDir)
	}
	if dp.Lod3 != Lod3DataDir {
		t.Errorf("Lod3 = %q, want %q (no country suffix)", dp.Lod3, Lod3DataDir)
	}
}

func TestCountryDataDir(t *testing.T) {
	sep := string(os.PathSeparator)
	tests := []struct {
		country, want string
		wantErr       bool
	}{
		{"germany", "data" + sep + "lod2" + sep + "germany", false},
		{"united_kingdom", "data" + sep + "lod2" + sep + "united_kingdom", false},
		{"..", "", true},
		{"../../etc", "", true},
		{"", "", true},
		{"germany/../../x", "", true},
	}
	for _, tc := range tests {
		got, err := countryDataDir(Lod2DataDir, tc.country)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("countryDataDir(%q) = %q, %v; want %q, error %v", tc.country, got, err, tc.want, tc.wantErr)
		}
	}
}
