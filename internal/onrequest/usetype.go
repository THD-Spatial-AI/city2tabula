package onrequest

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// useTypes is the neutral use vocabulary served as use_type. It must match the
// use_type enum in docs/openapi/openapi.yaml.
var useTypes = []string{
	"residential", "office", "retail", "hospitality", "education", "health",
	"public", "commercial", "industry", "agriculture", "ancillary", "other",
}

// usetype/<country>.csv maps a country's source function codes to a use_type and
// whether the building contains dwellings. Adding a country means adding its file.
//
//go:embed usetype/*.csv
var useTypeFiles embed.FS

type useClass struct {
	useType     string
	residential bool
}

// useTypeLists holds every embedded list by country, parsed once at start-up.
var useTypeLists = mustLoadUseTypeLists()

func mustLoadUseTypeLists() map[string]map[string]*useClass {
	lists, err := loadUseTypeLists()
	if err != nil {
		panic(err)
	}
	return lists
}

// loadUseTypeLists parses each embedded list. A row with an empty use_type maps
// its code to null, for codes that state the use is unknown.
func loadUseTypeLists() (map[string]map[string]*useClass, error) {
	files, err := useTypeFiles.ReadDir("usetype")
	if err != nil {
		return nil, fmt.Errorf("failed to list use type lists: %w", err)
	}
	lists := make(map[string]map[string]*useClass, len(files))
	for _, f := range files {
		path := "usetype/" + f.Name()
		data, err := useTypeFiles.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		list, err := parseUseTypeList(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		lists[strings.TrimSuffix(f.Name(), ".csv")] = list
	}
	return lists, nil
}

func parseUseTypeList(data string) (map[string]*useClass, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = 4
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	list := make(map[string]*useClass, len(rows))
	for _, row := range rows[1:] {
		code, useType, residential := row[0], row[1], row[2]
		if _, dup := list[code]; dup {
			return nil, fmt.Errorf("code %s listed twice", code)
		}
		if useType == "" {
			list[code] = nil
			continue
		}
		if !slices.Contains(useTypes, useType) {
			return nil, fmt.Errorf("code %s: use_type %q is not in the vocabulary", code, useType)
		}
		isResidential, err := strconv.ParseBool(residential)
		if err != nil {
			return nil, fmt.Errorf("code %s: residential %q: %w", code, residential, err)
		}
		list[code] = &useClass{useType: useType, residential: isResidential}
	}
	return list, nil
}

// useTypeFor returns the use_type and residential flag for a building's function
// code in country, both nil when the code is missing, unknown, or states no use.
func useTypeFor(country string, code *string) (*string, *bool) {
	if code == nil {
		return nil, nil
	}
	c := useTypeLists[country][*code]
	if c == nil {
		return nil, nil
	}
	useType, residential := c.useType, c.residential
	return &useType, &residential
}

// unitUse is one entry of the building_use JSON that buildingUseColumn returns.
type unitUse struct {
	DatasetID string  `json:"dataset_id"`
	Code      string  `json:"code"`
	Area      float64 `json:"area"`
}

// useTypeFromUnits returns the use type covering the most floor area among a
// building's units, whether any unit is a dwelling, and the datasets the mapped
// units came from. Equal areas go to the use type first by name, so the result
// does not depend on row order. Codes without a mapping in country are ignored,
// and all three results are nil when no unit has a mapped code.
func useTypeFromUnits(country string, units []byte) (*string, *bool, []string, error) {
	if len(units) == 0 {
		return nil, nil, nil, nil
	}
	var entries []unitUse
	if err := json.Unmarshal(units, &entries); err != nil {
		return nil, nil, nil, fmt.Errorf("decode building_use units: %w", err)
	}

	areas := map[string]float64{}
	residential := false
	var datasets []string
	for _, u := range entries {
		c := useTypeLists[country][u.Code]
		if c == nil {
			continue
		}
		areas[c.useType] += u.Area
		residential = residential || c.residential
		datasets = append(datasets, u.DatasetID)
	}
	if len(areas) == 0 {
		return nil, nil, nil, nil
	}

	best := ""
	for _, t := range slices.Sorted(maps.Keys(areas)) {
		if best == "" || areas[t] > areas[best] {
			best = t
		}
	}
	slices.Sort(datasets)
	return &best, &residential, slices.Compact(datasets), nil
}
