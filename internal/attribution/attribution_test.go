package attribution

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// validFields is a complete, valid attribution file as a map, so each test
// case can change one field.
func validFields() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"dataset_id":     "cz-brno-lod2",
		"provider":       "Example Provider",
		"dataset":        "Example LoD2 buildings",
		"licence":        "CC-BY-4.0",
		"licence_url":    "https://creativecommons.org/licenses/by/4.0/",
		"credit":         "Example Provider, example.org",
		"credit_url":     "https://example.org/",
		"terms_url":      "https://example.org/terms",
		"changes":        "City2TABULA derives building attributes from the geometry.",
	}
}

// writeFile writes content as attribution.json in a new directory and returns
// that directory.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", FileName, err)
	}
	return dir
}

func writeFields(t *testing.T, fields map[string]any) string {
	t.Helper()
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal fields: %v", err)
	}
	return writeFile(t, string(data))
}

func TestLoad_Valid(t *testing.T) {
	a, err := Load(writeFields(t, validFields()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Attribution{
		SchemaVersion: 1,
		DatasetID:     "cz-brno-lod2",
		Provider:      "Example Provider",
		Dataset:       "Example LoD2 buildings",
		Licence:       "CC-BY-4.0",
		LicenceURL:    "https://creativecommons.org/licenses/by/4.0/",
		Credit:        "Example Provider, example.org",
		CreditURL:     "https://example.org/",
		TermsURL:      "https://example.org/terms",
		Changes:       "City2TABULA derives building attributes from the geometry.",
	}
	if a != want {
		t.Errorf("Load = %+v, want %+v", a, want)
	}
}

func TestLoad_AcceptsVariants(t *testing.T) {
	tests := map[string]func(map[string]any){
		"LicenseRef licence":    func(f map[string]any) { f["licence"] = "LicenseRef-TABULA-EPISCOPE" },
		"German licence id":     func(f map[string]any) { f["licence"] = "DL-DE-BY-2.0" },
		"or-later licence id":   func(f map[string]any) { f["licence"] = "GPL-2.0+" },
		"id without lod suffix": func(f map[string]any) { f["dataset_id"] = "tabula-episcope" },
		"no credit_url":         func(f map[string]any) { delete(f, "credit_url") },
		"no terms_url":          func(f map[string]any) { delete(f, "terms_url") },
		"http url":              func(f map[string]any) { f["licence_url"] = "http://www.episcope.eu/" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			fields := validFields()
			change(fields)
			if _, err := Load(writeFields(t, fields)); err != nil {
				t.Errorf("Load: %v", err)
			}
		})
	}
}

func TestLoad_RejectsInvalidField(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{"wrong schema_version", func(f map[string]any) { f["schema_version"] = 2 }, "schema_version is 2, want 1"},
		{"missing schema_version", func(f map[string]any) { delete(f, "schema_version") }, "schema_version is 0, want 1"},
		{"uppercase dataset_id", func(f map[string]any) { f["dataset_id"] = "CZ-Brno-LoD2" }, `dataset_id "CZ-Brno-LoD2"`},
		{"dataset_id with underscore", func(f map[string]any) { f["dataset_id"] = "cz_brno" }, `dataset_id "cz_brno"`},
		{"dataset_id trailing hyphen", func(f map[string]any) { f["dataset_id"] = "cz-brno-" }, `dataset_id "cz-brno-"`},
		{"missing provider", func(f map[string]any) { delete(f, "provider") }, "provider is required"},
		{"blank dataset", func(f map[string]any) { f["dataset"] = "  " }, "dataset is required"},
		{"missing credit", func(f map[string]any) { delete(f, "credit") }, "credit is required"},
		{"missing changes", func(f map[string]any) { delete(f, "changes") }, "changes is required"},
		{"licence as prose", func(f map[string]any) { f["licence"] = "CC BY 4.0" }, `licence "CC BY 4.0"`},
		{"missing licence", func(f map[string]any) { delete(f, "licence") }, `licence ""`},
		{"missing licence_url", func(f map[string]any) { delete(f, "licence_url") }, "licence_url is required"},
		{"relative licence_url", func(f map[string]any) { f["licence_url"] = "creativecommons.org/licenses/by/4.0/" }, `licence_url "creativecommons.org`},
		{"ftp credit_url", func(f map[string]any) { f["credit_url"] = "ftp://example.org/" }, `credit_url "ftp://example.org/"`},
		{"hostless terms_url", func(f map[string]any) { f["terms_url"] = "https:///terms" }, `terms_url "https:///terms"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := validFields()
			tc.change(fields)
			dir := writeFields(t, fields)
			_, err := Load(dir)
			assertErrorContains(t, err, tc.want, filepath.Join(dir, FileName))
		})
	}
}

func TestLoad_RejectsMalformedFile(t *testing.T) {
	valid, err := json.Marshal(validFields())
	if err != nil {
		t.Fatalf("marshal fields: %v", err)
	}
	tests := []struct {
		name, content, want string
	}{
		{"misspelt key", strings.Replace(string(valid), `"credit_url"`, `"credit_link"`, 1), `unknown field "credit_link"`},
		{"two objects", string(valid) + string(valid), "unexpected data after the attribution object"},
		{"not JSON", "dataset_id: cz-brno-lod2", "parse JSON"},
		{"string for schema_version", strings.Replace(string(valid), `"schema_version":1`, `"schema_version":"1"`, 1), "parse JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeFile(t, tc.content)
			_, err := Load(dir)
			assertErrorContains(t, err, tc.want, filepath.Join(dir, FileName))
		})
	}
}

func TestLoad_MissingFileNamesPath(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	assertErrorContains(t, err, "no such file", filepath.Join(dir, FileName))
}

func TestValidate_ReportsEveryProblem(t *testing.T) {
	err := Attribution{}.Validate()
	if err == nil {
		t.Fatal("Validate of an empty Attribution returned nil")
	}
	for _, want := range []string{
		"schema_version", "dataset_id", "provider is required", "dataset is required",
		"credit is required", "changes is required", "licence", "licence_url is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestSchemaMatchesStruct fails when the published JSON Schema and the
// Attribution struct disagree on a field or on which fields are required.
func TestSchemaMatchesStruct(t *testing.T) {
	data, err := os.ReadFile("../../docs/code/attribution/attribution.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	var fields, required []string
	typ := reflect.TypeFor[Attribution]()
	for i := range typ.NumField() {
		name, opts, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		fields = append(fields, name)
		if opts != "omitempty" {
			required = append(required, name)
		}
	}

	if got := slices.Sorted(maps.Keys(schema.Properties)); !slices.Equal(got, slices.Sorted(slices.Values(fields))) {
		t.Errorf("schema properties %v, struct fields %v", got, fields)
	}
	if got := slices.Sorted(slices.Values(schema.Required)); !slices.Equal(got, slices.Sorted(slices.Values(required))) {
		t.Errorf("schema required %v, struct required %v", got, required)
	}
}

func assertErrorContains(t *testing.T, err error, want, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load returned nil error, want one containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the file %s", err, path)
	}
}
