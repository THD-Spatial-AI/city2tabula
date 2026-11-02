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

// regionEntry describes one region folder. Bbox is [min_lon, min_lat,
// max_lon, max_lat] in WGS84 degrees.
type regionEntry struct {
	Name string    `json:"name"`
	Bbox []float64 `json:"bbox"`
}

// checkRegions checks a dataset folder's regions.json against its model
// folders: every entry has a name, a WGS84 bbox and a folder, every region
// folder has an entry, and no model file lies outside a region folder. A
// dataset folder without regions.json passes.
func checkRegions(dir string, inputs []importInput) []error {
	r, ok, err := loadRegions(dir)
	if err != nil {
		return []error{err}
	}
	if !ok {
		return nil
	}

	path := filepath.Join(dir, regionsFileName)
	problems := r.problems(path)
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
	for _, slug := range slices.Sorted(maps.Keys(r.Regions)) {
		if !found[slug] {
			problems = append(problems, fmt.Errorf("%s: region %q has no folder in gml/ or cityjson/", path, slug))
		}
	}
	return problems
}

// loadRegions reads the regions.json in dir; ok is false when there is none.
func loadRegions(dir string) (r regionsFile, ok bool, err error) {
	path := filepath.Join(dir, regionsFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return regionsFile{}, false, nil
	}
	if err != nil {
		return regionsFile{}, false, fmt.Errorf("read regions file: %w", err)
	}
	r, err = decodeRegions(data)
	if err != nil {
		return regionsFile{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return r, true, nil
}

// problems reports every rule r breaks on its own, each naming path.
func (r regionsFile) problems(path string) []error {
	var problems []error
	if r.SchemaVersion != regionsSchemaVersion {
		problems = append(problems, fmt.Errorf("%s: schema_version is %d, want %d", path, r.SchemaVersion, regionsSchemaVersion))
	}
	for _, slug := range slices.Sorted(maps.Keys(r.Regions)) {
		e := r.Regions[slug]
		if strings.TrimSpace(e.Name) == "" {
			problems = append(problems, fmt.Errorf("%s: region %q has no name", path, slug))
		}
		if err := checkBbox(e.Bbox); err != nil {
			problems = append(problems, fmt.Errorf("%s: region %q: %w", path, slug, err))
		}
	}
	return problems
}

// checkBbox requires [min_lon, min_lat, max_lon, max_lat] in WGS84 degrees,
// each minimum below its maximum.
func checkBbox(b []float64) error {
	if len(b) != 4 {
		return fmt.Errorf("bbox has %d numbers, want [min_lon, min_lat, max_lon, max_lat]", len(b))
	}
	if b[0] < -180 || b[2] > 180 || b[1] < -90 || b[3] > 90 || b[0] >= b[2] || b[1] >= b[3] {
		return fmt.Errorf("bbox %v is not [min_lon, min_lat, max_lon, max_lat] in WGS84 degrees", b)
	}
	return nil
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
