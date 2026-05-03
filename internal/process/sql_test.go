package process

import (
	"reflect"
	"strconv"
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

// BenchmarkReplaceParameters measures the performance of replaceParameters function with a large SQL script and a large parameter map.
//
// Use case: In feature extraction, the SQL script can be quite large and the parameter map can contain many entries. This benchmark helps to understand how the function performs under such conditions and whether it can be optimized if needed.
func BenchmarkReplaceParameters(b *testing.B) {

	cases := []struct {
		name string
		size int
	}{
		// Case 1: small SQL script with 10 parameters
		{
			name: "given a small SQL script with 10 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			size: 10,
		},
		// Case 2: small SQL script with 100 parameters
		{
			name: "given a small SQL script with 100 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			size: 100,
		},
		// Case 3: medium SQL script with 1000 parameters
		{
			name: "given a medium SQL script with 1000 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			size: 1000,
		},
		// Case 4: large SQL script with 10000 parameters
		{
			name: "given a large SQL script with 10000 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			size: 10000,
		},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			// Create a large string with 1000 placeholders and a corresponding parameter map
			sqlScript := "SELECT * FROM {param1}"
			params := make(map[string]any)
			for i := 1; i <= bc.size; i++ {
				placeholder := "{param" + strconv.Itoa(i) + "}"
				sqlScript += " JOIN " + placeholder + " ON condition" + strconv.Itoa(i)
				params["param"+strconv.Itoa(i)] = "table" + strconv.Itoa(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				replaceParameters(sqlScript, params)
			}
		})
	}
}

func BenchmarkReplaceParameters_ScriptLength(b *testing.B) {
	params := map[string]any{
		"lod_schema":           "lod2",
		"srid":                 "25832",
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
		"building_ids":         "(1,2,3)",
	}

	cases := []struct {
		name   string
		script string
	}{
		{
			name:   "given a short script (~120 chars), when replaceParameters is called with 13 params, then returns updated script",
			script: "SELECT * FROM {lod_schema}.{tabula_table} WHERE srid = {srid} AND country = '{country}' AND building_id IN {building_ids};",
		},
		{
			name:   "given a medium script (~200 chars), when replaceParameters is called with 13 params, then returns updated script",
			script: "SELECT * FROM {lod_schema}.{tabula_table} JOIN {citydb_schema}.building ON b.id = t.id WHERE srid = {srid} AND country = '{country}' AND lod_level = {lod_level} AND building_id IN {building_ids};",
		},
		{
			name:   "given a long script (~350 chars), when replaceParameters is called with 13 params, then returns updated script",
			script: "SELECT * FROM {lod_schema}.{tabula_table} JOIN {citydb_schema}.building ON b.id = t.id JOIN {citydb_pkg_schema}.package ON p.id = b.id WHERE srid = {srid} AND country = '{country}' AND lod_level = {lod_level} AND room_height > {room_height} AND city2tabula_schema = '{city2tabula_schema}' AND building_id IN {building_ids};",
		},
		{
			name:   "given a very long script (~550 chars), when replaceParameters is called with 13 params, then returns updated script",
			script: "SELECT b.id, b.geom, t.tabula_class FROM {lod_schema}.{tabula_table} AS t JOIN {citydb_schema}.building AS b ON b.id = t.id JOIN {citydb_pkg_schema}.package AS p ON p.id = b.id JOIN {public_schema}.spatial_ref_sys AS s ON s.srid = {srid} WHERE s.srid = {srid} AND t.country = '{country}' AND b.lod_level = {lod_level} AND b.room_height > {room_height} AND t.schema = '{city2tabula_schema}' AND t.tabula_variant IN (SELECT id FROM {tabula_schema}.{tabula_variant_table}) AND b.id IN {building_ids};",
		},
	}
	// loop + b.ResetTimer() + replaceParameters call
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				replaceParameters(bc.script, params)
			}
		})
	}
}

// Benchmark 2 — param count, script length fixed:

func BenchmarkReplaceParameters_ParamCount(b *testing.B) {
	// fixed script — one placeholder per param you plan to test
	// keep the script the same length across all cases

	cases := []struct {
		name   string
		params map[string]any
	}{
		// fill in: 5 params, 13 params (realistic), 50 params
		{
			name: "given a SQL script with 5 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			params: map[string]any{
				"lod_schema":   "lod2",
				"srid":         "25832",
				"country":      "DEU",
				"building_ids": "(1,2,3)",
				"tabula_table": "tabula",
			},
		},
		{
			name: "given a SQL script with 13 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			params: map[string]any{
				"lod_schema":           "lod2",
				"srid":                 "25832",
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
			name: "given a SQL script with 50 parameters, when replaceParameters is called, then returns updated script with all parameters replaced",
			params: func() map[string]any {
				params := make(map[string]any)
				for i := 1; i <= 50; i++ {
					params["param"+strconv.Itoa(i)] = "value" + strconv.Itoa(i)
				}
				return params
			}(),
		},
	}
	// loop + b.ResetTimer() + replaceParameters call
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			// create a SQL script with 50 placeholders matching the params in the largest case
			sqlScript := "SELECT * FROM {lod_schema}.{tabula_table} WHERE srid = {srid} AND country = '{country}' AND building_id IN {building_ids}"
			for i := 1; i <= 50; i++ {
				sqlScript += " AND param" + strconv.Itoa(i) + " = {" + "param" + strconv.Itoa(i) + "}"
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				replaceParameters(sqlScript, bc.params)
			}
		})
	}
}
