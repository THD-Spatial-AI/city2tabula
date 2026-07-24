package config

import "testing"

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
