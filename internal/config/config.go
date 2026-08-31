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

	// Central PyLovo database reached via postgres_fdw for -link-pylovo
	PylovoFDW *PylovoFDW

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

	cfg := Config{
		Country:     country,
		CountryCode: code,
		DB:          loadDBConfig(code),
		Data:        loadDataPaths(),
		CityDB:      loadCityDBConfig(country),
		City2Tabula: loadCity2TabulaConfig(),
		PylovoFDW:   loadPylovoFDWConfig(),
		Batch:       loadBatchConfig(),
		RetryConfig: DefaultRetryConfig(),
	}

	if cfg.PylovoFDW.Enabled() {
		// FDW mode: -link-pylovo reads pylovo.res/oth as foreign tables imported
		// into this fixed schema, so PYLOVO_SCHEMA no longer applies.
		cfg.DB.Schemas.Pylvo = PylvoFDWSchemaName
	}

	return cfg
}

// getCountry returns the normalized country name
func getCountry() string {
	return strings.ToLower(normalizeCountryName(GetEnv("COUNTRY", "")))
}
