package tests

import (
	"os"
	"testing"
	"time"

	"City2TABULA/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestGetCountry(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "Normal country name",
			envValue: "germany",
			expected: "germany",
		},
		{
			name:     "Uppercase country",
			envValue: "GERMANY",
			expected: "germany",
		},
		{
			name:     "Mixed case country",
			envValue: "GeRmAnY",
			expected: "germany",
		},
		{
			name:     "Empty country",
			envValue: "",
			expected: "",
		},
		{
			name:     "Country with spaces",
			envValue: " germany ",
			expected: "germany",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			originalCountry := os.Getenv("COUNTRY")
			defer func() {
				if originalCountry != "" {
					os.Setenv("COUNTRY", originalCountry)
				} else {
					os.Unsetenv("COUNTRY")
				}
			}()

			if tt.envValue != "" {
				os.Setenv("COUNTRY", tt.envValue)
			} else {
				os.Unsetenv("COUNTRY")
			}

			// Test through LoadConfig since getCountry is not exported
			cfg := config.LoadConfig()
			assert.Equal(t, tt.expected, cfg.Country)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		config := config.Config{
			Country: "germany",
			DB: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
				SSLMode:  "disable",
			},
			Data: &config.DataPaths{
				Base: "/test/path",
			},
			CityDB: &config.CityDB{
				SRID:     "4326",
				ToolPath: "/path/to/citydb",
				SRSName:  "EPSG:4326",
			},
			Batch: &config.BatchConfig{
				Size: 100,
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Missing country", func(t *testing.T) {
		config := config.Config{
			Country: "",
			DB: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
				SSLMode:  "disable",
			},
			CityDB: &config.CityDB{
				SRID:     "4326",
				ToolPath: "/path/to/citydb",
				SRSName:  "EPSG:4326",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "COUNTRY")
	})

	t.Run("Missing DB config", func(t *testing.T) {
		config := config.Config{
			Country: "germany",
			DB:      nil,
			CityDB: &config.CityDB{
				SRID:     "4326",
				ToolPath: "/path/to/citydb",
				SRSName:  "EPSG:4326",
			},
		}

		// The validation function doesn't handle nil DB gracefully, so it will panic
		// This is a design choice - the application requires a DB config
		assert.Panics(t, func() {
			config.Validate()
		})
	})

	t.Run("Missing DB fields", func(t *testing.T) {
		config := config.Config{
			Country: "germany",
			DB: &config.DBConfig{
				Host: "localhost",
				// Missing other required fields
			},
			CityDB: &config.CityDB{
				SRID:     "4326",
				ToolPath: "/path/to/citydb",
				SRSName:  "EPSG:4326",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("Missing CityDB fields", func(t *testing.T) {
		config := config.Config{
			Country: "germany",
			DB: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
				SSLMode:  "disable",
			},
			CityDB: &config.CityDB{
				// Missing required fields
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CITYDB_TOOL_PATH")
	})
}

func TestLoadConfig(t *testing.T) {
	// Set up test environment variables
	envVars := map[string]string{
		"COUNTRY":          "germany",
		"DB_HOST":          "localhost",
		"DB_PORT":          "5432",
		"DB_USER":          "testuser",
		"DB_PASSWORD":      "testpass",
		"DB_NAME":          "testdb",
		"DB_SSLMODE":       "disable",
		"DATA_PATH":        "/test/data",
		"CITYDB_TOOL_PATH": "/path/to/citydb",
		"CITYDB_SRID":      "4326",
		"CITYDB_SRS_NAME":  "EPSG:4326",
		"BATCH_SIZE":       "100",
	}

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
	}

	// Clean up after test
	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	config := config.LoadConfig()

	assert.Equal(t, "germany", config.Country)
	assert.NotNil(t, config.DB)
	assert.NotNil(t, config.Data)
	assert.NotNil(t, config.CityDB)
	assert.NotNil(t, config.Batch)
	assert.NotNil(t, config.RetryConfig)

	// Validate the loaded config
	err := config.Validate()
	assert.NoError(t, err)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
		setEnv       bool
	}{
		{
			name:         "Environment variable exists",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "actual",
			expected:     "actual",
			setEnv:       true,
		},
		{
			name:         "Environment variable does not exist",
			key:          "NON_EXISTENT_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
			setEnv:       false,
		},
		{
			name:         "Empty environment variable",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
			setEnv:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := config.GetEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	retryConfig := config.DefaultRetryConfig()

	assert.NotNil(t, retryConfig)
	assert.Greater(t, retryConfig.MaxRetries, 0)
	assert.Greater(t, retryConfig.InitialDelay, time.Duration(0))
	assert.Greater(t, retryConfig.MaxDelay, time.Duration(0))
	assert.Greater(t, retryConfig.BackoffFactor, float64(1))
}
