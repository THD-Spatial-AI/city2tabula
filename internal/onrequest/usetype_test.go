package onrequest

import "testing"

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
