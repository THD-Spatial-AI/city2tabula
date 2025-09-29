package config

import (
	"fmt"
	"path/filepath"
	"slices"
)

// SQL directory constants
const (
	SQLDir = "sql/"

	// Script directories
	SQLScriptDir              = SQLDir + "scripts/"
	SQLMainScriptDir          = SQLScriptDir + "main/"          // Core feature extraction pipeline
	SQLSupplementaryScriptDir = SQLScriptDir + "supplementary/" // Supporting/setup scripts

	// Schema files
	SQLSchemaFileDir = SQLDir + "schema/"

	// Function files
	SQLTrainingFunctionsPath = SQLDir + "functions/"
)

// SQLScripts holds dynamically loaded SQL script paths
type SQLScripts struct {
	MainScripts          []string // Core feature extraction pipeline (01-10)
	SupplementaryScripts []string // Supporting scripts (tabula extraction, etc.)
	TableScripts         []string // Schema creation scripts
	FunctionScripts      []string // Function scripts
}

// SQLParameters holds all SQL template parameters
type SQLParameters struct {
	BuildingIDs        []int64 `param:"building_ids"`
	LodSchema          string  `param:"lod_schema"`
	SRID               string  `param:"srid"`
	City2TabulaSchema  string  `param:"city2tabula_schema"`
	TabulaSchema       string  `param:"tabula_schema"`
	LodLevel           int     `param:"lod_level"`
	PublicSchema       string  `param:"public_schema"`
	CityDBSchema       string  `param:"citydb_schema"`
	CityDBPkgSchema    string  `param:"citydb_pkg_schema"`
	Country            string  `param:"country"`
	TabulaTable        string  `param:"tabula_table"`
	TabulaVariantTable string  `param:"tabula_variant_table"`
}

// GetSQLParameters returns SQL parameters for a specific LOD level
func (c *Config) GetSQLParameters(lod int, buildingIDs []int64) SQLParameters {
	lodSchema := ""
	if lod == 2 {
		lodSchema = c.DB.Schemas.Lod2
	} else if lod == 3 {
		lodSchema = c.DB.Schemas.Lod3
	}

	return SQLParameters{
		BuildingIDs:        buildingIDs,
		LodSchema:          lodSchema,
		SRID:               c.CityDB.SRID,
		City2TabulaSchema:  c.DB.Schemas.City2Tabula,
		TabulaSchema:       c.DB.Schemas.Tabula,
		LodLevel:           lod,
		PublicSchema:       c.DB.Schemas.Public,
		CityDBSchema:       c.DB.Schemas.CityDB,
		CityDBPkgSchema:    c.DB.Schemas.CityDBPkg,
		Country:            c.Country,
		TabulaTable:        c.DB.Tables.Tabula,
		TabulaVariantTable: c.DB.Tables.TabulaVariant,
	}
}

func (c *Config) LoadSQLScripts() (*SQLScripts, error) {
	// Load main scripts
	mainScripts, err := loadSQLFilesFromDir(SQLMainScriptDir)
	if err != nil {
		return nil, err
	}

	// Load supplementary scripts
	supplementaryScripts, err := loadSQLFilesFromDir(SQLSupplementaryScriptDir)
	if err != nil {
		return nil, err
	}

	// Load schema scripts
	TableScripts, err := loadSQLFilesFromDir(SQLSchemaFileDir)
	if err != nil {
		return nil, err
	}

	// Load function scripts
	functionScripts, err := loadSQLFilesFromDir(SQLTrainingFunctionsPath)
	if err != nil {
		return nil, err
	}
	return &SQLScripts{
		MainScripts:          mainScripts,
		SupplementaryScripts: supplementaryScripts,
		TableScripts:         TableScripts,
		FunctionScripts:      functionScripts,
	}, nil
}

func loadSQLFilesFromDir(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	if files == nil {
		return nil, fmt.Errorf("no SQL files found in directory: %s", dir)
	}

	var sqlFiles []string
	for _, file := range files {
		filename := filepath.Base(file)
		sqlFiles = append(sqlFiles, filepath.Join(dir, filename))
	}

	// Sort files to ensure consistent order using prefix numbers
	// e.g., 01_init.sql, 02_extract.sql, etc.
	// This assumes files are named with leading numbers for ordering
	slices.Sort(sqlFiles)
	return sqlFiles, nil
}
