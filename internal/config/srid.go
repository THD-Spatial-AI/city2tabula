package config

import "fmt"

// sridInfo pairs a target SRID with its human-readable SRS name, matching the
// CITYDB_SRID / CITYDB_SRS_NAME pair a user sets by hand in .env today.
type sridInfo struct {
	srid    string
	srsName string
}

// sridByCountry mirrors the reference table in .env.example's comments — same key
// set as isoByCountry (TABULA-supported countries only; Switzerland is listed in
// .env.example but isn't a TABULA country, so it's intentionally omitted here too).
// Needed so the HTTP server (internal/api) can derive CITYDB_SRID/CITYDB_SRS_NAME
// per request instead of requiring them as static env vars, the way the CLI does.
var sridByCountry = map[string]sridInfo{
	"austria":        {"31256", "MGI / Austria GK East"},
	"belgium":        {"31370", "Belgian Lambert 72"},
	"bulgaria":       {"7801", "BGS2005 / CCS2005"},
	"cyprus":         {"3879", "GRS 1980 / Cyprus TM"},
	"czechia":        {"5514", "S-JTSK / Krovak East North"},
	"denmark":        {"25832", "ETRS89 / UTM zone 32N"},
	"france":         {"2154", "RGF93 / Lambert-93"},
	"germany":        {"25832", "ETRS89 / UTM zone 32N"},
	"greece":         {"2100", "GGRS87 / Greek Grid"},
	"hungary":        {"23700", "EOV"},
	"ireland":        {"29902", "Irish National Grid"},
	"italy":          {"3003", "Monte Mario / Italy zone 1"},
	"netherlands":    {"28992", "Amersfoort / RD New"},
	"norway":         {"25833", "ETRS89 / UTM zone 33N"},
	"serbia":         {"3114", "Serbian 1970 / Serbian Grid"},
	"slovenia":       {"3794", "Slovenia 1996 / Slovene National Grid"},
	"poland":         {"2180", "ETRS89 / Poland CS2000 zone 5"},
	"spain":          {"25830", "ETRS89 / UTM zone 30N"},
	"sweden":         {"3006", "SWEREF99 TM"},
	"united_kingdom": {"27700", "OSGB 1936 / British National Grid"},
}

// SRIDForCountry returns the target SRID and SRS name for the given normalized
// country name, or an error if the country is not in the TABULA dataset.
func SRIDForCountry(country string) (srid, srsName string, err error) {
	info, ok := sridByCountry[country]
	if !ok {
		return "", "", fmt.Errorf("unsupported country %q: no SRID reference available", country)
	}
	return info.srid, info.srsName, nil
}
