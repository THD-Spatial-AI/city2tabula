package importer

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func TestResolveCRS(t *testing.T) {
	tests := []struct {
		name    string
		want    int
		wantErr bool
	}{
		{"urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH", 25832, false}, // Bavaria, NRW
		{"urn:ogc:def:crs,crs:EPSG::31256", 31256, false},         // Vienna
		{"urn:ogc:def:crs:EPSG::25832", 25832, false},
		{"https://www.opengis.net/def/crs/EPSG/0/5514", 5514, false},  // Brno
		{"https://www.opengis.net/def/crs/EPSG/0/7415", 28992, false}, // 3DBAG, compound
		{"EPSG:28992", 28992, false},
		{"EPSG:74150", 74150, false},
		{"unknown", 0, true},                    // Prague
		{"urn:ogc:drf:crs:EPSG::3879", 0, true}, // Helsinki, malformed
		{"", 0, true},
	}
	for _, tc := range tests {
		got, err := resolveCRS(tc.name)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("resolveCRS(%q) = %d, %v; want %d, error %v", tc.name, got, err, tc.want, tc.wantErr)
		}
	}
}

func gmlTile(srsName string) string {
	return `<?xml version="1.0"?><core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0">` +
		`<gml:boundedBy><gml:Envelope srsName="` + srsName + `" srsDimension="3"/></gml:boundedBy></core:CityModel>`
}

func writeFile(t *testing.T, path string, content string, gzipped bool) {
	t.Helper()
	data := []byte(content)
	if gzipped {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		data = b.Bytes()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDataset(t *testing.T) {
	const adv = "urn:adv:crs:ETRS89_UTM32*DE_DHHN2016_NH"
	tests := []struct {
		name    string
		country string
		setup   func(t *testing.T, staged, published string)
		wantErr []string
	}{
		{"valid gzipped CityGML in a region subfolder", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "region-a", "tile.gml.gz"), gmlTile(adv), true)
		}, nil},
		{"CityJSON with metadata after a long CityObjects", "netherlands", func(t *testing.T, s, _ string) {
			body := `{"CityObjects":{"a":{"attributes":{"pad":"` + strings.Repeat("x", 1_500_000) + `"}}},` +
				`"metadata":{"referenceSystem":"https://www.opengis.net/def/crs/EPSG/0/7415"},"type":"CityJSON"}`
			writeFile(t, filepath.Join(s, "cityjson", "tile.city.json.gz"), body, true)
		}, nil},
		{"CRS of another country", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile("urn:ogc:def:crs,crs:EPSG::31256"), false)
		}, []string{"EPSG:31256", "EPSG:25832"}},
		{"CRS not a recognised name", "czechia", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile("unknown"), false)
		}, []string{`"unknown"`}},
		{"no CRS declared", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), `<core:CityModel></core:CityModel>`, false)
		}, []string{"declares no srsName"}},
		{"not CityGML", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), `<html></html>`, false)
		}, []string{"is not CityGML"}},
		{"model file outside the model folders", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile(adv), false)
			writeFile(t, filepath.Join(s, "extra", "tile.gml"), gmlTile(adv), false)
		}, []string{"extra", "would not be imported"}},
		{"hidden folders are not read", "germany", func(t *testing.T, s, _ string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile(adv), false)
			writeFile(t, filepath.Join(s, ".venv", "lib", "schema.json"), "{}", false)
		}, nil},
		{"dataset_id used by a published folder with another attribution", "germany", func(t *testing.T, s, p string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile(adv), false)
			other := writeDataset(t, p, "other", "de-test-lod2")
			data, _ := os.ReadFile(filepath.Join(other, "attribution.json"))
			writeFile(t, filepath.Join(other, "attribution.json"), strings.Replace(string(data), "Example Provider,", "Another Provider,", 1), false)
		}, []string{`dataset_id "de-test-lod2" is already used`}},
		{"dataset_id of the published folder it extends", "germany", func(t *testing.T, s, p string) {
			writeFile(t, filepath.Join(s, "gml", "tile.gml"), gmlTile(adv), false)
			writeDataset(t, p, "test", "de-test-lod2")
		}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stagingRoot, published := t.TempDir(), t.TempDir()
			staged := writeDataset(t, stagingRoot, "staged", "de-test-lod2")
			tc.setup(t, staged, published)
			cfg := discoverConfig(published, filepath.Join(published, "no-lod3"))
			cfg.Country = tc.country

			problems := CheckDataset(cfg, staged)
			var all []string
			for _, p := range problems {
				all = append(all, p.Error())
			}
			joined := strings.Join(all, "\n")
			if len(tc.wantErr) == 0 && len(problems) > 0 {
				t.Fatalf("want no problems, got:\n%s", joined)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(joined, want) {
					t.Errorf("problems do not mention %q:\n%s", want, joined)
				}
			}
		})
	}
}

func TestCheckDataset_UsesConfiguredSRID(t *testing.T) {
	staged := writeDataset(t, t.TempDir(), "staged", "de-test-lod2")
	writeFile(t, filepath.Join(staged, "gml", "tile.gml"), gmlTile("urn:ogc:def:crs:EPSG::25833"), false)
	cfg := discoverConfig(t.TempDir(), t.TempDir())
	cfg.Country = "germany"
	if problems := CheckDataset(cfg, staged); len(problems) == 0 {
		t.Fatal("EPSG:25833 passed against the German default EPSG:25832")
	}
	cfg.CityDB = &config.CityDB{SRID: "25833"}
	if problems := CheckDataset(cfg, staged); len(problems) != 0 {
		t.Errorf("with CITYDB_SRID 25833: %v", problems)
	}
}

func TestCheckDataset_MissingAttributionAndModelFolders(t *testing.T) {
	staged := t.TempDir()
	cfg := discoverConfig(t.TempDir(), t.TempDir())
	cfg.Country = "germany"
	joined := ""
	for _, p := range CheckDataset(cfg, staged) {
		joined += p.Error() + "\n"
	}
	for _, want := range []string{"attribution", "neither a gml nor a cityjson"} {
		if !strings.Contains(joined, want) {
			t.Errorf("problems do not mention %q:\n%s", want, joined)
		}
	}
}
