package config

import "strings"

// Table name constants
const (
	Tabula        = "tabula"
	TabulaVariant = "tabula_variant"
)

// Schema name constants
const (
	PublicSchema      = "public"
	CityDBSchema      = "citydb"
	CityDBPkgSchema   = "citydb_pkg"
	Lod2Schema        = "lod2"
	Lod3Schema        = "lod3"
	TabulaSchema      = "tabula"
	City2TabulaSchema = "city2tabula"
	PylvoSchemaName   = "public" // default PyLovo schema; override with PYLOVO_SCHEMA env var
)

// Tables holds all table name configurations
type Tables struct {
	City2Tabula       string
	ThreeDCity2Tabula string
	Tabula            string
	TabulaVariant     string
}

// Schemas holds all schema configurations
type Schemas struct {
	Public      string
	CityDB      string
	CityDBPkg   string
	Lod2        string
	Lod3        string
	Tabula      string
	City2Tabula string
	Pylvo       string // PyLovo schema containing res and oth tables
}

// Database configuration
type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string

	// Database structure
	Tables  *Tables
	Schemas *Schemas
	SQL     *SQLScripts
}

// loadDBConfig loads database configuration from environment. countryCode isolates
// each country's raw 3DCityDB data in its own database (see dbNameForCountry).
func loadDBConfig(countryCode string) *DBConfig {
	return &DBConfig{
		Host:     GetEnv("DB_HOST", "localhost"),
		Port:     GetEnv("DB_PORT", "5432"),
		Name:     dbNameForCountry(GetEnv("DB_NAME", ""), countryCode),
		User:     GetEnv("DB_USER", "postgres"),
		Password: GetEnv("DB_PASSWORD", ""),
		SSLMode:  GetEnv("DB_SSL_MODE", ""),

		// Database structure
		Tables:  loadTables(),
		Schemas: loadSchemas(),
		SQL:     nil,
	}
}

// dbNameForCountry suffixes name with the country's ISO2 code (e.g. "city2tabula_de").
// 3DCityDB stores one SRS per database, and different countries use different
// national CRSs, so each country gets its own database rather than sharing one.
// Falls back to the bare name when name or countryCode is empty, so an unsupported
// country (caught later by Validate()) never produces a trailing "_".
func dbNameForCountry(name, countryCode string) string {
	if name == "" || countryCode == "" {
		return name
	}
	return name + "_" + strings.ToLower(countryCode)
}

// loadSchemas loads schema configuration
func loadSchemas() *Schemas {
	return &Schemas{
		Public:      PublicSchema,
		CityDB:      CityDBSchema,
		CityDBPkg:   CityDBPkgSchema,
		Lod2:        Lod2Schema,
		Lod3:        Lod3Schema,
		Tabula:      TabulaSchema,
		City2Tabula: City2TabulaSchema,
		Pylvo:       GetEnv("PYLOVO_SCHEMA", PylvoSchemaName),
	}
}

func (s *Schemas) All() []string {
	return []string{
		s.Public,
		s.CityDB,
		s.CityDBPkg,
		s.Lod2,
		s.Lod3,
		s.Tabula,
		s.City2Tabula,
	}
}

// loadTables loads table configuration
func loadTables() *Tables {
	return &Tables{
		Tabula:        Tabula,
		TabulaVariant: TabulaVariant,
	}
}
