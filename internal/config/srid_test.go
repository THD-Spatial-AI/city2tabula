package config

import "testing"

func TestSRIDForCountry_SameKeySetAsCountryCode(t *testing.T) {
	for country := range isoByCountry {
		t.Run(country, func(t *testing.T) {
			srid, srsName, err := SRIDForCountry(country)
			if err != nil {
				t.Errorf("SRIDForCountry(%q) unexpected error: %v", country, err)
			}
			if srid == "" {
				t.Errorf("SRIDForCountry(%q) returned empty srid", country)
			}
			if srsName == "" {
				t.Errorf("SRIDForCountry(%q) returned empty srsName", country)
			}
		})
	}
}

func TestSRIDForCountry_Germany(t *testing.T) {
	srid, srsName, err := SRIDForCountry("germany")
	if err != nil {
		t.Fatalf("SRIDForCountry(\"germany\") unexpected error: %v", err)
	}
	if srid != "25832" {
		t.Errorf("srid = %q, want %q", srid, "25832")
	}
	if srsName != "ETRS89 / UTM zone 32N" {
		t.Errorf("srsName = %q, want %q", srsName, "ETRS89 / UTM zone 32N")
	}
}

func TestSRIDForCountry_UnsupportedCountry(t *testing.T) {
	_, _, err := SRIDForCountry("atlantis")
	if err == nil {
		t.Error("SRIDForCountry(\"atlantis\") expected error, got nil")
	}
}
