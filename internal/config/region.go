package config

import "strings"

// LoadBaseConfig loads the process-wide parts of configuration that don't vary
// per request: DB connection, CityDB tool path, thread count, batch/retry
// settings. Country, CountryCode, DB.Name, Data.Lod2/Lod3, and CityDB.SRID/SRSName
// are left unset here — RegionConfig fills them in per request. Used by the HTTP
// server (internal/api), which serves many countries from one long-running
// process, unlike the CLI's single COUNTRY-per-invocation model.
func LoadBaseConfig() Config {
	LoadEnv()
	return Config{
		DB:          loadDBConfig(""),
		Data:        loadDataPaths(),
		CityDB:      loadCityDBConfig(""),
		City2Tabula: loadCity2TabulaConfig(),
		PylovoFDW:   loadPylovoFDWConfig(),
		Batch:       loadBatchConfig(),
		RetryConfig: DefaultRetryConfig(),
	}
}

// RegionConfig derives a full, independent Config for one country from a
// process-wide base (see LoadBaseConfig). Each call returns fresh DB and CityDB
// structs so concurrent requests for different countries never share mutable
// state — the HTTP server may be building configs for several countries at once.
func RegionConfig(base Config, country string) (Config, error) {
	normalized := strings.ToLower(normalizeCountryName(country))

	code, err := CountryCode(normalized)
	if err != nil {
		return Config{}, err
	}
	srid, srsName, err := SRIDForCountry(normalized)
	if err != nil {
		return Config{}, err
	}

	db := *base.DB
	db.Name = dbNameForCountry(base.DB.Name, code)
	schemas := *base.DB.Schemas
	db.Schemas = &schemas

	// Same switch LoadConfig makes for the CLI: in FDW mode -link-pylovo reads
	// pylovo.res/oth as foreign tables in this fixed schema, so PYLOVO_SCHEMA no
	// longer applies. Without it a server-triggered run finds no PyLovo tables
	// and produces no building_link rows.
	if base.PylovoFDW.Enabled() {
		schemas.Pylvo = PylvoFDWSchemaName
	}

	cityDB := *base.CityDB
	cityDB.SRID = srid
	cityDB.SRSName = srsName

	return Config{
		Country:     normalized,
		CountryCode: code,
		DB:          &db,
		Data: &DataPaths{
			Base:   base.Data.Base,
			Lod2:   Lod2DataDir + normalized,
			Lod3:   Lod3DataDir + normalized,
			Tabula: base.Data.Tabula,
		},
		CityDB:      &cityDB,
		City2Tabula: base.City2Tabula,
		PylovoFDW:   base.PylovoFDW,
		Batch:       base.Batch,
		RetryConfig: base.RetryConfig,
	}, nil
}
