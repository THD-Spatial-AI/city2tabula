package onrequest

import "testing"

func TestBbox_String(t *testing.T) {
	b := Bbox{Xmin: 11.5, Ymin: 48.5, Xmax: 12.0, Ymax: 49.0}
	want := "11.5,48.5,12,49,4326"
	if got := b.String(); got != want {
		t.Errorf("Bbox.String() = %q, want %q", got, want)
	}
}
