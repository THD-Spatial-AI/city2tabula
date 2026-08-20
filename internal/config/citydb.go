package config

import (
	"path/filepath"
	"strconv"
)

// CityDB configuration
type CityDB struct {
	SRSName     string
	ToolPath    string
	SRID        string
	LODLevels   []int
	ImportLimit int // 0 = no limit

	// CityDB SQL Scripts for database setup
	SQLScripts struct {
		CreateDB     string
		CreateSchema string
		DropDB       string
		DropSchema   string
	}
}

// loadCityDBConfig loads CityDB configuration. country (normalized, e.g.
// "germany") derives SRID/SRSName from SRIDForCountry when CITYDB_SRID/
// CITYDB_SRS_NAME aren't set explicitly — country is "" for the process-wide
// base config (see LoadBaseConfig), which has no SRID to derive yet.
func loadCityDBConfig(country string) *CityDB {
	cityDBToolPath := GetEnv("CITYDB_TOOL_PATH", "")
	cityDBSRSName := GetEnv("CITYDB_SRS_NAME", "")
	cityDBSRID := GetEnv("CITYDB_SRID", "")
	if cityDBSRID == "" && cityDBSRSName == "" && country != "" {
		if srid, srsName, err := SRIDForCountry(country); err == nil {
			cityDBSRID = srid
			cityDBSRSName = srsName
		}
	}
	cityDBLODLevels := []int{2, 3}

	importLimit := 0
	if v := GetEnv("IMPORT_LIMIT", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			importLimit = n
		}
	}

	return &CityDB{
		SRSName:     cityDBSRSName,
		ToolPath:    cityDBToolPath,
		SRID:        cityDBSRID,
		LODLevels:   cityDBLODLevels,
		ImportLimit: importLimit,

		SQLScripts: struct {
			CreateDB     string
			CreateSchema string
			DropDB       string
			DropSchema   string
		}{
			CreateDB:     filepath.Join(cityDBToolPath, "3dcitydb", "postgresql", "sql-scripts", "create-db.sql"),
			CreateSchema: filepath.Join(cityDBToolPath, "3dcitydb", "postgresql", "sql-scripts", "create-schema.sql"),
			DropDB:       filepath.Join(cityDBToolPath, "3dcitydb", "postgresql", "sql-scripts", "drop-db.sql"),
			DropSchema:   filepath.Join(cityDBToolPath, "3dcitydb", "postgresql", "sql-scripts", "drop-schema.sql"),
		},
	}
}
