package config

import (
	"strings"
	"testing"
)

// Use-case: User configures the .env file with predefined environment variable. Program tries to read the variable using predefined keys. It should get the correct value defined by the user in .env
//
// Case 1:
// Given: Empty string is provided as key
// When: GetEnv is called, it does not find provided key in .env
// Then: Returns fallback value
func TestGetEnv(t *testing.T) {
	cases := []struct {
		name     string
		envKey   string
		envValue string
		fallback string
		want     string
	}{
		{
			name:     "given key is empty string, when called, then returns fallback value",
			envKey:   "",
			envValue: "",
			fallback: "fallback",
			want:     "fallback",
		},
		{
			name:     "given key exists with value, when called, then returns env value",
			envKey:   "TEST",
			envValue: "test",
			fallback: "fallback",
			want:     "test",
		},
		{
			name:     "given key exists without value, when called, then returns fallback value",
			envKey:   "TEST",
			envValue: "",
			fallback: "fallback",
			want:     "fallback",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {

			// Given
			if testCase.envValue != "" {
				t.Setenv(testCase.envKey, testCase.envValue)
			}

			// When
			got := GetEnv(testCase.envKey, testCase.fallback)

			// Then
			if got != testCase.want {
				t.Errorf("expected %v, got %v", testCase.want, got)
			}
		})
	}
}

// Use-case: User configures .env file with a integer value, but by default it's a string. Program requires the value as integer, when GetEnvAsInt called with key it returns the value as integrer instead of string
//
// Case 1:
// Given: Key exists with integer value as string in .env
// When: GetEnvAsInt is called, it finds the key and converts the value to integer
// Then: Returns the integer value
//
// Case 2:
// Given: Key exists without value in .env
// When: GetEnvAsInt is called, it finds the key but value is empty string
// Then: Returns fallback integer value
//
// Case 3:
// Given: Key does not exist in .env
// When: GetEnvAsInt is called, it does not find the key in .env
// Then: Returns fallback integer value
func TestGetEnvAsInt(t *testing.T) {
	cases := []struct {
		name     string
		envKey   string
		envValue string
		fallback int
		want     int
	}{
		{
			name:     "given no env key is provide, when called, then returns fallback",
			envKey:   "",
			envValue: "",
			fallback: 0,
			want:     0,
		},
		{
			name:     "given valid key is provided, when called, then returns an integer value",
			envKey:   "TEST",
			envValue: "5",
			fallback: 0,
			want:     5,
		},
		{
			name:     "given valid key is provided but value is not set, when called, then returns fallback",
			envKey:   "TEST",
			envValue: "",
			fallback: 0,
			want:     0,
		},
		{
			name:     "given valid key with non-numeric value is provided,when called, then returns fallback",
			envKey:   "TEST",
			envValue: "test",
			fallback: 0,
			want:     0,
		},
		{
			name:     "given valid key with negative integer is provided, when called, then returns negative integer",
			envKey:   "TEST",
			envValue: "-5",
			fallback: 0,
			want:     -5,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {

			// Given
			if testCase.envValue != "" {
				t.Setenv(testCase.envKey, testCase.envValue)
			}

			// When
			got := GetEnvAsInt(testCase.envKey, testCase.fallback)

			// Then
			if got != testCase.want {
				t.Errorf("want %d, got %d", testCase.want, got)
			}
		})
	}
}

func TestNormalizeCountryName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "given empty string as input, when called, then returns empty string",
			input: "",
			want:  "",
		},
		{
			name:  "given input containing capital letters, when called, then returns same string in all small letters",
			input: "TesT",
			want:  "test",
		},
		{
			name:  "given input contains space, when called, then returns same string with underscore instead of space",
			input: "Test Input",
			want:  "test_input",
		},
		{
			name:  "given input contains hyphen, when called, then returns value with underscore instead of hyphen",
			input: "Test-inPut",
			want:  "test_input",
		},
		{
			name:  "given input contains trailing white spaces, when called, then returns value without and trailing white spaces",
			input: " United-Kingdom ",
			want:  "united_kingdom",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			// (input defined in table above)

			// When
			got := normalizeCountryName(testCase.input)

			// Then
			if got != testCase.want {
				t.Errorf("want %q, got %q", testCase.want, got)
			}
		})
	}

}

func TestValidate(t *testing.T) {

	cases := []struct {
		name            string
		config          Config
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "given all required fields are set, when Validate is called, then returns nil",
			config: Config{
				Country: "germany",
				DB: &DBConfig{
					Host:     "localhost",
					Port:     "5432",
					Name:     "city2tabula",
					User:     "postgres",
					Password: "secret",
				},
				CityDB: &CityDB{
					ToolPath: "/opt/citydb-tool",
					SRID:     "25832",
					SRSName:  "EPSG:25832",
				},
			},
			wantErr: false,
		},
		{
			name: "given DB_NAME is whitespace only, when Validate is called, then error contains DB_NAME",
			config: Config{
				Country: "germany",
				DB: &DBConfig{
					Host:     "localhost",
					Port:     "5432",
					Name:     "   ", // spaces only
					User:     "postgres",
					Password: "secret",
				},
				CityDB: &CityDB{
					ToolPath: "/opt/citydb-tool",
					SRID:     "25832",
					SRSName:  "EPSG:25832",
				},
			},
			wantErr:         true,
			wantErrContains: "DB_NAME",
		},
		{
			name: "given multiple fields missing, when Validate is called, then error contains all missing field names",
			config: Config{
				DB:     &DBConfig{Host: "localhost", Port: "5432", User: "postgres"},
				CityDB: &CityDB{SRID: "25832", SRSName: "EPSG:25832"},
			},
			wantErr:         true,
			wantErrContains: "DB_NAME",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			err := testCase.config.Validate()

			// Then
			if (err != nil) != testCase.wantErr {
				t.Errorf("wantErr %v, got %v", testCase.wantErr, err)
			}
			if testCase.wantErr && !strings.Contains(err.Error(), testCase.wantErrContains) {
				t.Errorf("wantErrContains %q, got %q", testCase.wantErrContains, err.Error())
			}
		})
	}
}
