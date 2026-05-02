package process

import (
	"reflect"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// TestGetSQLParameterMap verifies that getSQLParameterMap correctly converts struct into a map containing key value pair. The key and corresponding value should match the source data.
//
// Use case: replaceParameter get the map of SQL parameter registered in config package, allowing it to find the parameter by key and replace it with the corresponding 'param' value.
//
// Case 1: Happy path
// Given: SQLParameters struct with fields tagged with 'param' and corresponding values
// When: getSQLParameterMap is called with the struct
// Then: Returns a map where each key is the 'param' tag value and each value is the corresponding field value from the struct.
//
// Case 2: Empty struct
// Given: An empty SQLParameters struct (all fields are zero values)
// When: getSQLParameterMap is called with the empty struct
// Then: Returns a map where each key is the 'param' tag value and each value is the zero value of the corresponding field type.
//
// Case 3: Struct with some fields set to zero values
// Given: A SQLParameters struct where some fields have non-zero values and others are zero values
// When: getSQLParameterMap is called with this struct
// Then: Returns a map where keys correspond to all 'param' tags, with values reflecting both non-zero and zero field values as appropriate.
func TestGetSQLParameterMap(t *testing.T) {
	cases := []struct {
		name   string
		params config.SQLParameters
		want   map[string]any
	}{
		{
			name: "given a struct with param tags, when getSQLParameterMap is called, then returns map with correct key value pairs",
			params: config.SQLParameters{
				BuildingIDs:        []int64{1, 2, 3},
				LodSchema:          "lod2",
				SRID:               "4326",
				City2TabulaSchema:  "city2tabula",
				TabulaSchema:       "tabula",
				LodLevel:           2,
				PublicSchema:       "public",
				CityDBSchema:       "citydb",
				CityDBPkgSchema:    "citydb_pkg",
				Country:            "DEU",
				TabulaTable:        "tabula",
				TabulaVariantTable: "tabula_variant",
				RoomHeight:         "3.0",
			},
			want: map[string]any{
				"building_ids":         []int64{1, 2, 3},
				"lod_schema":           "lod2",
				"srid":                 "4326",
				"city2tabula_schema":   "city2tabula",
				"tabula_schema":        "tabula",
				"lod_level":            2,
				"public_schema":        "public",
				"citydb_schema":        "citydb",
				"citydb_pkg_schema":    "citydb_pkg",
				"country":              "DEU",
				"tabula_table":         "tabula",
				"tabula_variant_table": "tabula_variant",
				"room_height":          "3.0",
			},
		},
		{
			name:   "given an empty struct, when getSQLParameterMap is called, then returns map with zero values",
			params: config.SQLParameters{},
			want: map[string]any{
				"building_ids":         []int64(nil),
				"lod_schema":           "",
				"srid":                 "",
				"city2tabula_schema":   "",
				"tabula_schema":        "",
				"lod_level":            0,
				"public_schema":        "",
				"citydb_schema":        "",
				"citydb_pkg_schema":    "",
				"country":              "",
				"tabula_table":         "",
				"tabula_variant_table": "",
				"room_height":          "",
			},
		},
		{
			name: "given a struct with some fields set to zero values, when getSQLParameterMap is called, then returns map reflecting both non-zero and zero values",
			params: config.SQLParameters{
				BuildingIDs:        []int64{1, 2, 3},
				LodSchema:          "lod2",
				SRID:               "4326",
				City2TabulaSchema:  "city2tabula",
				TabulaSchema:       "tabula",
				LodLevel:           2,
				PublicSchema:       "public",
				CityDBSchema:       "citydb",
				CityDBPkgSchema:    "citydb_pkg",
				Country:            "DEU",
				TabulaTable:        "tabula",
				TabulaVariantTable: "tabula_variant",
			},
			want: map[string]any{
				"building_ids":         []int64{1, 2, 3},
				"lod_schema":           "lod2",
				"srid":                 "4326",
				"city2tabula_schema":   "city2tabula",
				"tabula_schema":        "tabula",
				"lod_level":            2,
				"public_schema":        "public",
				"citydb_schema":        "citydb",
				"citydb_pkg_schema":    "citydb_pkg",
				"country":              "DEU",
				"tabula_table":         "tabula",
				"tabula_variant_table": "tabula_variant",
				"room_height":          "",
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := getSQLParameterMap(testCase.params)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("expected %v, got %v", testCase.want, got)
			}
		})
	}
}

// TestReplaceParameters verifies that replaceParameters correctly replaces the SQL parameters in provide SQL script as string with corresponding value.
//
// Use case: Worker provides the SQL script as string and map of parameters which contains all parameters predefined by user in config package. Each parameter placeholder is then replaced with corresponding value and then worker executes the script. The function must replace the parameter with correct value, respecting SQL Grammar rules, handling edge cases gracefully.
//
// Case 1: Happy path
// Given: SQL script is passed into function as string along with parameter map correctly
// When: All parameter placeholders matching with keys in parameter map are replaced with corresponding values.
// Then: updated script is returned as string.
//
// Case 2
// Given: SQL script does not contain any placeholder '{}' provided in the param map
// When: Function does not find any param listed in map
// Then: Returns script unchanged
//
// Case 3
// Given: SQL script contains parameter which is not listed in the map
// When: Function does not find any param listed in map
// Then: Returns script unchanged
//
// Case 4
// Given: a SQL script and parameter map are empty
// When: replaceParameter is called
// Then: Returns script as it is
func TestReplaceParameters(t *testing.T) {
	cases := []struct {
		name      string
		sqlScript string
		params    map[string]any
		want      string
	}{
		{
			name:      "given empty script and empty params map, when called, then returns empty string",
			sqlScript: "",
			params:    map[string]any{},
			want:      "",
		},
		{
			name:      "given correct script and correct params map, when called, then returns updated script with param values",
			sqlScript: "SELECT * FROM {param1}.{param2};",
			params:    map[string]any{"param1": "a", "param2": "b"},
			want:      "SELECT * FROM a.b;",
		},
		{
			name:      "given script without placeholder and correct params, when called, then returns script unchanged",
			sqlScript: "SELECT * FROM a;",
			params:    map[string]any{"lodSchema": "lod2", "c2tSchema": "city2tabula"},
			want:      "SELECT * FROM a;",
		},
		{
			name:      "given script with placeholder which is not listed in params map, when called, then returns script with placeholder unchanged",
			sqlScript: "SELECT {unknown_param} FROM {known_param};",
			params:    map[string]any{"known_param": "building"},
			want:      "SELECT {unknown_param} FROM building;",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {

			got := replaceParameters(testCase.sqlScript, testCase.params)

			if got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
