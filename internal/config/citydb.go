package config

import "path/filepath"

// CityDB configuration
type CityDB struct {
	SRSName   string
	ToolPath  string
	SRID      string
	LODLevels []int

	// CityDB SQL Scripts for database setup
	SQLScripts struct {
		CreateDB     string
		CreateSchema string
		DropDB       string
		DropSchema   string
	}
}

// loadCityDBConfig loads CityDB configuration
func loadCityDBConfig() *CityDB {
	cityDBToolPath := GetEnv("CITYDB_TOOL_PATH", "")
	cityDBSRSName := GetEnv("CITYDB_SRS_NAME", "")
	cityDBSRID := GetEnv("CITYDB_SRID", "")
	cityDBLODLevels := []int{2, 3}

	return &CityDB{
		SRSName:   cityDBSRSName,
		ToolPath:  cityDBToolPath,
		SRID:      cityDBSRID,
		LODLevels: cityDBLODLevels,

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
