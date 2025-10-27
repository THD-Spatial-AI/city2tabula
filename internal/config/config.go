package config

import (
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

	// City2TABULA settings
	City2Tabula *City2TabulaConfig

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
		City2Tabula: loadCity2TabulaConfig(),
		Batch:       loadBatchConfig(),
		RetryConfig: DefaultRetryConfig(),
	}
}

// getCountry returns the normalized country name
func getCountry() string {
	return strings.ToLower(normalizeCountryName(GetEnv("COUNTRY", "")))
}
