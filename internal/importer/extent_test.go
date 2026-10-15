package importer

import "testing"

func TestParseImportBbox(t *testing.T) {
	tests := []struct {
		in      string
		want    importBbox
		wantErr bool
	}{
		{in: "16.3,48.2,16.4,48.3,4326", want: importBbox{16.3, 48.2, 16.4, 48.3, 4326}},
		{in: "690000,5330000,691000,5331000", want: importBbox{690000, 5330000, 691000, 5331000, 25832}},
		{in: "1,2,3", wantErr: true},
		{in: "1,2,x,4", wantErr: true},
		{in: "1,2,3,4,epsg", wantErr: true},
	}
	for _, tc := range tests {
		got, err := parseImportBbox(tc.in, "25832")
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseImportBbox(%q): want error, got %+v", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseImportBbox(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
		}
	}
}
