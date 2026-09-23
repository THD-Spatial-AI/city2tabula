package flags

import (
	"flag"
	"os"
	"strings"
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
		{"import-data", func(f *Flags) bool { return f.ImportData }},
		{"reset-db", func(f *Flags) bool { return f.ResetDB }},
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

	if f.CreateDB || f.ResetDB || f.ResetC2T || f.ImportData ||
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

// The guidance printed when -create-db meets an existing database is the only
// place a user holding new source files is told what to run instead. Offering
// reset-db ahead of -import-data there discards every extracted feature and
// hand correction in the database.
func TestCreateDBGuidanceOffersImportBeforeReset(t *testing.T) {
	msg := AllMessages.CreateDB.Custom

	importIdx := strings.Index(msg, "-import-data")
	if importIdx == -1 {
		t.Fatal("expected the already-exists guidance to name -import-data")
	}
	resetIdx := strings.Index(msg, "reset-db")
	if resetIdx == -1 {
		t.Fatal("expected the already-exists guidance to still name reset-db")
	}
	if resetIdx < importIdx {
		t.Error("expected -import-data to be offered before reset-db")
	}
}
