package config

import "fmt"

// isoByCountry maps the normalized TABULA country name (value of COUNTRY env var)
// to its ISO 3166-1 alpha-2 code. Only countries with TABULA data are supported.
var isoByCountry = map[string]string{
	"austria":        "AT",
	"belgium":        "BE",
	"bulgaria":       "BG",
	"cyprus":         "CY",
	"czechia":        "CZ",
	"denmark":        "DK",
	"france":         "FR",
	"germany":        "DE",
	"greece":         "GR",
	"hungary":        "HU",
	"ireland":        "IE",
	"italy":          "IT",
	"netherlands":    "NL",
	"norway":         "NO",
	"poland":         "PL",
	"serbia":         "RS",
	"slovenia":       "SI",
	"spain":          "ES",
	"sweden":         "SE",
	"united_kingdom": "GB",
}

// CountryCode returns the ISO 3166-1 alpha-2 code for the given normalized
// country name, or an error if the country is not in the TABULA dataset.
func CountryCode(country string) (string, error) {
	code, ok := isoByCountry[country]
	if !ok {
		return "", fmt.Errorf("unsupported country %q: no TABULA data available", country)
	}
	return code, nil
}
