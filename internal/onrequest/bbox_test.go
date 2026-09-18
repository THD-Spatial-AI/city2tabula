package onrequest

import "testing"

func TestBboxOverlaps(t *testing.T) {
	// A 10x10 box at the origin, against boxes placed around it.
	base := Bbox{Xmin: 0, Ymin: 0, Xmax: 10, Ymax: 10}

	tests := []struct {
		name  string
		other Bbox
		want  bool
	}{
		{"identical", Bbox{0, 0, 10, 10}, true},
		{"fully contained", Bbox{2, 2, 8, 8}, true},
		{"contains base", Bbox{-5, -5, 15, 15}, true},
		{"partial corner", Bbox{5, 5, 15, 15}, true},
		{"touching right edge", Bbox{10, 0, 20, 10}, true},
		{"touching corner", Bbox{10, 10, 20, 20}, true},
		{"disjoint in x", Bbox{10.5, 0, 20, 10}, false},
		{"disjoint in y", Bbox{0, 10.5, 10, 20}, false},
		{"disjoint in both", Bbox{20, 20, 30, 30}, false},
		{"adjacent in x, overlapping y", Bbox{-10, 5, -0.5, 15}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.Overlaps(tt.other); got != tt.want {
				t.Errorf("base.Overlaps(%v) = %v, want %v", tt.other, got, tt.want)
			}
			// Overlap is symmetric; a one-directional check would let a
			// caller's argument order decide whether two runs serialise.
			if got := tt.other.Overlaps(base); got != tt.want {
				t.Errorf("%v.Overlaps(base) = %v, want %v (not symmetric)", tt.other, got, tt.want)
			}
		})
	}
}
