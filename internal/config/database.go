package config

import "fmt"

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

// loadDBConfig loads database configuration from environment
func loadDBConfig() *DBConfig {
	country := normalizeCountryName(GetEnv("COUNTRY", ""))
	return &DBConfig{
		Host:     GetEnv("DB_HOST", "localhost"),
		Port:     GetEnv("DB_PORT", "5432"),
		Name:     fmt.Sprintf("city2tabula_%s", country),
		User:     GetEnv("DB_USER", "postgres"),
		Password: GetEnv("DB_PASSWORD", ""),
		SSLMode:  GetEnv("DB_SSL_MODE", ""),

		// Database structure
		Tables:  loadTables(),
		Schemas: loadSchemas(),
		SQL:     nil,
	}
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
