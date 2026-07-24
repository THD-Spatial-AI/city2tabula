package flags

import (
	"flag"
	"os"
	"testing"
)

// resetFlags gives ParseFlags a fresh, empty flag.CommandLine before each
// case - flag.BoolVar panics ("flag redefined") if the same flag name is
// registered twice on the same FlagSet, which every ParseFlags call does.
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func TestParseFlags_EachFlagSetsItsOwnField(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	cases := []struct {
		flagName string
		get      func(f *Flags) bool
	}{
		{"create-db", func(f *Flags) bool { return f.CreateDB }},
		{"reset-db", func(f *Flags) bool { return f.ResetDB }},
		{"reset-citydb", func(f *Flags) bool { return f.ResetCityDB }},
		{"reset-city2tabula", func(f *Flags) bool { return f.ResetC2T }},
		{"extract-features", func(f *Flags) bool { return f.ExtractFeatures }},
		{"link-pylovo", func(f *Flags) bool { return f.LinkPylovo }},
		{"version", func(f *Flags) bool { return f.ShowVersion }},
		{"v", func(f *Flags) bool { return f.ShowV }},
	}

	for _, tc := range cases {
		t.Run(tc.flagName, func(t *testing.T) {
			resetFlags()
			os.Args = []string{"city2tabula", "-" + tc.flagName}

			f := ParseFlags()

			if !tc.get(f) {
				t.Errorf("-%s: expected corresponding field to be true, got false", tc.flagName)
			}
			// Then: every other field stays at its false default.
			for _, other := range cases {
				if other.flagName == tc.flagName {
					continue
				}
				if other.get(f) {
					t.Errorf("-%s: expected -%s to remain false, got true", tc.flagName, other.flagName)
				}
			}
		})
	}
}

func TestParseFlags_NoFlagsAllDefaultFalse(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	resetFlags()
	os.Args = []string{"city2tabula"}

	f := ParseFlags()

	if f.CreateDB || f.ResetDB || f.ResetCityDB || f.ResetC2T ||
		f.ExtractFeatures || f.LinkPylovo || f.ShowVersion || f.ShowV {
		t.Errorf("expected every flag to default to false with no args, got %+v", f)
	}
}

func TestParseFlags_MultipleFlagsCombine(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	resetFlags()
	os.Args = []string{"city2tabula", "-extract-features", "-link-pylovo"}

	f := ParseFlags()

	if !f.ExtractFeatures || !f.LinkPylovo {
		t.Errorf("expected both -extract-features and -link-pylovo to be true, got %+v", f)
	}
	if f.CreateDB || f.ResetDB {
		t.Errorf("expected unrelated flags to stay false, got %+v", f)
	}
}
