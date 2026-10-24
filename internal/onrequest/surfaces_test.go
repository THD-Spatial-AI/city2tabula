package onrequest

import "testing"

func TestSurfaceCursorRoundTrip(t *testing.T) {
	for _, id := range []string{"DEBY_LOD2_7672144", "NL.IMBAG.Pand.0150100000000150", "UUID_LOD2_000019-b1f7acdd"} {
		got, err := DecodeSurfaceCursor(EncodeSurfaceCursor(id))
		if err != nil || got != id {
			t.Errorf("round trip of %q = %q, %v", id, got, err)
		}
	}
	if got, err := DecodeSurfaceCursor(""); err != nil || got != "" {
		t.Errorf("empty cursor = %q, %v; want the first page", got, err)
	}
	if _, err := DecodeSurfaceCursor("%%"); err == nil {
		t.Error("malformed cursor decoded without error")
	}
}
