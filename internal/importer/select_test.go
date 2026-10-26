package importer

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileExtent(t *testing.T) {
	dir := t.TempDir()
	envelope := `<core:CityModel><gml:boundedBy><gml:Envelope srsName="urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH">` +
		`<gml:lowerCorner srsDimension="3">786000.0 5416000.0 0.0</gml:lowerCorner>` +
		`<gml:upperCorner srsDimension="3">788000.0 5418000.0 400.0</gml:upperCorner></gml:Envelope></gml:boundedBy></core:CityModel>`
	cityJSON := `{"type":"CityJSON","CityObjects":{"b":{"geographicalExtent":[1,2,3,4,5,6],"attributes":{"pad":"` +
		strings.Repeat("x", 1_200_000) + `"}}},"metadata":{"geographicalExtent":[197420.5,457109.6,61.1,197431.0,457120.3,68.5],` +
		`"referenceSystem":"https://www.opengis.net/def/crs/EPSG/0/7415"}}`
	tests := []struct {
		name, file, content, format string
		want                        fileExtent
		wantOK                      bool
	}{
		{"CityGML envelope, corners with attributes, gzipped", "a.gml.gz", envelope, "citygml",
			fileExtent{786000, 5416000, 788000, 5418000, "urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH"}, true},
		{"CityJSON extent from metadata, not from a CityObject", "b.city.json.gz", cityJSON, "cityjson",
			fileExtent{197420.5, 457109.6, 197431.0, 457120.3, "https://www.opengis.net/def/crs/EPSG/0/7415"}, true},
		{"CityGML without an envelope", "c.gml", `<core:CityModel></core:CityModel>`, "citygml", fileExtent{}, false},
		{"CityJSON metadata without an extent", "d.city.json", `{"type":"CityJSON","metadata":{"referenceSystem":"EPSG:5514"},"CityObjects":{}}`, "cityjson", fileExtent{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.file)
			writeFile(t, path, tc.content, strings.HasSuffix(tc.file, ".gz"))
			got, ok, err := readFileExtent(path, tc.format)
			if err != nil || ok != tc.wantOK || got != tc.want {
				t.Errorf("readFileExtent = %+v, %v, %v; want %+v, %v", got, ok, err, tc.want, tc.wantOK)
			}
		})
	}
}
