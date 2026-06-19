package config

import "testing"

func TestCountryCode_SupportedCountries(t *testing.T) {
	cases := []struct {
		country string
		want    string
	}{
		{"austria", "AT"},
		{"belgium", "BE"},
		{"bulgaria", "BG"},
		{"cyprus", "CY"},
		{"czechia", "CZ"},
		{"denmark", "DK"},
		{"france", "FR"},
		{"germany", "DE"},
		{"greece", "GR"},
		{"hungary", "HU"},
		{"ireland", "IE"},
		{"italy", "IT"},
		{"netherlands", "NL"},
		{"norway", "NO"},
		{"poland", "PL"},
		{"serbia", "RS"},
		{"slovenia", "SI"},
		{"spain", "ES"},
		{"sweden", "SE"},
		{"united_kingdom", "GB"},
	}

	for _, tc := range cases {
		t.Run(tc.country, func(t *testing.T) {
			got, err := CountryCode(tc.country)
			if err != nil {
				t.Errorf("CountryCode(%q) unexpected error: %v", tc.country, err)
			}
			if got != tc.want {
				t.Errorf("CountryCode(%q) = %q, want %q", tc.country, got, tc.want)
			}
		})
	}
}

func TestCountryCode_UnsupportedCountry(t *testing.T) {
	_, err := CountryCode("atlantis")
	if err == nil {
		t.Error("CountryCode(\"atlantis\") expected error, got nil")
	}
}
