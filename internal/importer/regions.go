package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// regionsFileName is the optional file next to attribution.json that names the
// region folders under a dataset's gml/ and cityjson/ folders.
const regionsFileName = "regions.json"

// regionsSchemaVersion is the regions file format this package reads.
const regionsSchemaVersion = 1

// regionsFile is the content of one regions.json file, keyed by region folder name.
type regionsFile struct {
	SchemaVersion int                    `json:"schema_version"`
	Regions       map[string]regionEntry `json:"regions"`
}

// regionEntry describes one region folder.
type regionEntry struct {
	Name string `json:"name"`
}

// checkRegions checks a dataset folder's regions.json against its model
// folders: every region folder has an entry with a name, every entry has a
// folder, and no model file lies outside a region folder. A dataset folder
// without regions.json passes.
func checkRegions(dir string, inputs []importInput) []error {
	path := filepath.Join(dir, regionsFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []error{fmt.Errorf("read regions file: %w", err)}
	}
	r, err := decodeRegions(data)
	if err != nil {
		return []error{fmt.Errorf("%s: %w", path, err)}
	}

	var problems []error
	if r.SchemaVersion != regionsSchemaVersion {
		problems = append(problems, fmt.Errorf("%s: schema_version is %d, want %d", path, r.SchemaVersion, regionsSchemaVersion))
	}
	slugs := slices.Sorted(maps.Keys(r.Regions))
	for _, slug := range slugs {
		if strings.TrimSpace(r.Regions[slug].Name) == "" {
			problems = append(problems, fmt.Errorf("%s: region %q has no name", path, slug))
		}
	}
	found := map[string]bool{}
	for _, in := range inputs {
		entries, err := os.ReadDir(in.path)
		if err != nil {
			problems = append(problems, fmt.Errorf("read %s: %w", in.path, err))
			continue
		}
		for _, e := range entries {
			problems = append(problems, checkRegionEntry(path, in.path, e, r, found)...)
		}
	}
	for _, slug := range slugs {
		if !found[slug] {
			problems = append(problems, fmt.Errorf("%s: region %q has no folder in gml/ or cityjson/", path, slug))
		}
	}
	return problems
}

// checkRegionEntry checks one entry of a model folder when the dataset has a
// regions file: a folder must be listed in it, and a model file must lie in a
// region folder. Listed folders are recorded in found.
func checkRegionEntry(regionsPath, folder string, e os.DirEntry, r regionsFile, found map[string]bool) []error {
	path := filepath.Join(folder, e.Name())
	if strings.HasPrefix(e.Name(), ".") {
		return nil
	}
	if !e.IsDir() {
		if isModelFile(path) {
			return []error{fmt.Errorf("%s lies outside a region folder, which %s requires", path, regionsPath)}
		}
		return nil
	}
	if _, ok := r.Regions[e.Name()]; !ok {
		return []error{fmt.Errorf("%s has no entry in %s", path, regionsPath)}
	}
	found[e.Name()] = true
	return nil
}

// decodeRegions parses one JSON object, rejecting unknown fields so a misspelt
// key fails instead of leaving its field empty.
func decodeRegions(data []byte) (regionsFile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r regionsFile
	if err := dec.Decode(&r); err != nil {
		return regionsFile{}, fmt.Errorf("parse JSON: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return regionsFile{}, errors.New("parse JSON: unexpected data after the regions object")
	}
	if len(r.Regions) == 0 {
		return regionsFile{}, errors.New("regions is empty")
	}
	return r, nil
}
