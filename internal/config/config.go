package config

import (
	"fmt"
	"strings"
)

// Main Config holds the application configuration
type Config struct {
	// Global settings
	Country string

	// Database connection and structure
	DB *DBConfig

	// Dataset paths
	Data *DataPaths

	// CityDB configuration
	CityDB *CityDB

	// Batch processing
	Batch *BatchConfig

	// Retry configuration
	RetryConfig *RetryConfig
}

// LoadConfig is the single entry point for all configuration
func LoadConfig() Config {
	LoadEnv()

	return Config{
		Country:     getCountry(),
		DB:          loadDBConfig(),
		Data:        loadDataPaths(),
		CityDB:      loadCityDBConfig(),
		Batch:       loadBatchConfig(),
		RetryConfig: DefaultRetryConfig(),
	}
}

// getCountry returns the normalized country name
func getCountry() string {
	return strings.ToLower(normalizeCountryName(GetEnv("COUNTRY", "")))
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	missing := []string{}

	if strings.TrimSpace(c.DB.Host) == "" {
		missing = append(missing, "DB_HOST")
	}
	if strings.TrimSpace(c.DB.Port) == "" {
		missing = append(missing, "DB_PORT")
	}
	if strings.TrimSpace(c.DB.User) == "" {
		missing = append(missing, "DB_USER")
	}
	if strings.TrimSpace(c.DB.Password) == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if strings.TrimSpace(c.CityDB.ToolPath) == "" {
		missing = append(missing, "CITYDB_TOOL_PATH")
	}
	if strings.TrimSpace(c.CityDB.SRID) == "" {
		missing = append(missing, "CITYDB_SRID")
	}
	if strings.TrimSpace(c.CityDB.SRSName) == "" {
		missing = append(missing, "CITYDB_SRS_NAME")
	}
	if strings.TrimSpace(c.Country) == "" {
		missing = append(missing, "COUNTRY")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
