package config

import (
	"strings"
)

// Main Config holds the application configuration
type Config struct {
	// Global settings
	Country     string // normalized country name (e.g. "germany")
	CountryCode string // ISO 3166-1 alpha-2 code (e.g. "DE"); derived from Country

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

	country := getCountry()
	code, _ := CountryCode(country) // empty string if unsupported; Validate() will catch it

	return Config{
		Country:     country,
		CountryCode: code,
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
