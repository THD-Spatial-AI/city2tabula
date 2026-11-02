package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Data directory constants
const (
	DataDir       = "data" + string(os.PathSeparator)
	Lod2DataDir   = DataDir + "lod2" + string(os.PathSeparator)
	Lod3DataDir   = DataDir + "lod3" + string(os.PathSeparator)
	TabulaDataDir = DataDir + "tabula" + string(os.PathSeparator)
	// AttributesDataDir holds per-country building attributes that are not part
	// of the 3D model, such as building use from a national register.
	AttributesDataDir = DataDir + "attributes" + string(os.PathSeparator)
)

// Data paths
type DataPaths struct {
	Base       string
	Lod2       string
	Lod3       string
	Tabula     string
	Attributes string
}

// countryDataDir returns the country's folder under root, refusing a name that
// resolves outside root. The country comes from a server request; CountryCode's
// allowlist rejects such names first, and this keeps the path safe without it.
func countryDataDir(root, country string) (string, error) {
	dir := filepath.Join(root, country)
	if !strings.HasPrefix(dir, filepath.Clean(root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("country %q resolves outside the data directory %s", country, root)
	}
	return dir, nil
}

// DataCountries returns the country folder names under the LOD2 and LOD3 data
// directories, sorted and without duplicates. Hidden folders are skipped, and
// a missing data directory holds none.
func DataCountries() ([]string, error) {
	seen := map[string]bool{}
	for _, root := range []string{Lod2DataDir, Lod3DataDir} {
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read data directory %s: %w", root, err)
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				seen[e.Name()] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// loadDataPaths loads data directory paths
func loadDataPaths() *DataPaths {
	return &DataPaths{
		Base:       DataDir,
		Lod2:       Lod2DataDir + normalizeCountryName(GetEnv("COUNTRY", "")),
		Lod3:       Lod3DataDir + normalizeCountryName(GetEnv("COUNTRY", "")),
		Tabula:     TabulaDataDir,
		Attributes: AttributesDataDir + normalizeCountryName(GetEnv("COUNTRY", "")),
	}
}
