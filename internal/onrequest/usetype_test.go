package onrequest

import (
	"slices"
	"strings"
	"testing"
)

func TestUseTypeFor(t *testing.T) {
	code := func(s string) *string { return &s }
	tests := []struct {
		name            string
		country         string
		code            *string
		wantUse         string
		wantResidential bool
		wantNull        bool
	}{
		{"residential", "germany", code("31001_1010"), "residential", true, false},
		{"top-level public group", "germany", code("31001_3000"), "public", false, false},
		{"business with dwellings", "germany", code("31001_2310"), "commercial", true, false},
		{"carport outside the building codes", "germany", code("51009_1611"), "ancillary", false, false},
		{"code removed from the source list", "germany", code("31001_2465"), "ancillary", false, false},
		{"not specifiable from sources", "germany", code("31001_9998"), "", false, true},
		{"unknown code", "germany", code("31001_0000"), "", false, true},
		{"no code", "germany", nil, "", false, true},
		{"country without a list", "netherlands", code("31001_1010"), "", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			use, residential := useTypeFor(tc.country, tc.code)
			if tc.wantNull {
				if use != nil || residential != nil {
					t.Errorf("useTypeFor = %v, %v, want null, null", use, residential)
				}
				return
			}
			if use == nil || *use != tc.wantUse || residential == nil || *residential != tc.wantResidential {
				t.Errorf("useTypeFor = %v, %v, want %s, %v", use, residential, tc.wantUse, tc.wantResidential)
			}
		})
	}
}

// TestUseTypeListsLoad pins that every embedded list parses into the vocabulary,
// so a bad row fails here rather than when the server first serves that country.
func TestUseTypeListsLoad(t *testing.T) {
	lists, err := loadUseTypeLists()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(lists["germany"]); n < 300 {
		t.Errorf("germany list has %d codes, want the full AdV list", n)
	}
}

func TestParseUseTypeList_RejectsBadRows(t *testing.T) {
	const header = "code,use_type,residential,label\n"
	tests := []struct {
		name, rows, wantErr string
	}{
		{"duplicate code", "31001_1000,residential,true,a\n31001_1000,office,false,b\n", "listed twice"},
		{"use_type outside the vocabulary", "31001_1000,housing,true,a\n", "not in the vocabulary"},
		{"residential not a boolean", "31001_1000,residential,yes,a\n", "residential"},
		{"missing column", "31001_1000,residential,true\n", "wrong number of fields"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseUseTypeList(header + tc.rows)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("parseUseTypeList error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestUseTypeFromUnits(t *testing.T) {
	tests := []struct {
		name            string
		units           string
		wantUse         string
		wantResidential bool
		wantDatasets    []string
		wantNull        bool
	}{
		{"single use", `[{"dataset_id":"nl-bag-use","code":"woonfunctie","area":80}]`,
			"residential", true, []string{"nl-bag-use"}, false},
		{"largest floor area wins, dwellings still count", `[
			{"dataset_id":"nl-bag-use","code":"winkelfunctie","area":300},
			{"dataset_id":"nl-bag-use","code":"woonfunctie","area":120}]`,
			"retail", true, []string{"nl-bag-use"}, false},
		{"codes of one use type add up", `[
			{"dataset_id":"nl-bag-use","code":"sportfunctie","area":100},
			{"dataset_id":"nl-bag-use","code":"bijeenkomstfunctie","area":100},
			{"dataset_id":"nl-bag-use","code":"kantoorfunctie","area":150}]`,
			"public", false, []string{"nl-bag-use"}, false},
		{"equal areas pick the first use type by name", `[
			{"dataset_id":"nl-bag-use","code":"winkelfunctie","area":50},
			{"dataset_id":"nl-bag-use","code":"kantoorfunctie","area":50}]`,
			"office", false, []string{"nl-bag-use"}, false},
		{"unmapped codes are ignored", `[
			{"dataset_id":"nl-bag-use","code":"onbekend","area":900},
			{"dataset_id":"nl-bag-use","code":"onderwijsfunctie","area":10}]`,
			"education", false, []string{"nl-bag-use"}, false},
		{"building without units", `[{"dataset_id":"nl-bag-use","code":"geen verblijfsobject","area":0}]`,
			"ancillary", false, []string{"nl-bag-use"}, false},
		{"only unmapped codes", `[{"dataset_id":"nl-bag-use","code":"onbekend","area":10}]`, "", false, nil, true},
		{"no units", ``, "", false, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			use, residential, datasets, err := useTypeFromUnits("netherlands", []byte(tc.units))
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantNull {
				if use != nil || residential != nil || datasets != nil {
					t.Errorf("useTypeFromUnits = %v, %v, %v, want all null", use, residential, datasets)
				}
				return
			}
			if use == nil || *use != tc.wantUse || residential == nil || *residential != tc.wantResidential ||
				!slices.Equal(datasets, tc.wantDatasets) {
				t.Errorf("useTypeFromUnits = %v, %v, %v, want %s, %v, %v",
					use, residential, datasets, tc.wantUse, tc.wantResidential, tc.wantDatasets)
			}
		})
	}
}
