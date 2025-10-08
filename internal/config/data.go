package config

import "os"

// Data directory constants
const (
	DataDir       = "data" + string(os.PathSeparator)
	Lod2DataDir   = DataDir + "lod2" + string(os.PathSeparator)
	Lod3DataDir   = DataDir + "lod3" + string(os.PathSeparator)
	TabulaDataDir = DataDir + "tabula" + string(os.PathSeparator)
)

// Data paths
type DataPaths struct {
	Base   string
	Lod2   string
	Lod3   string
	Tabula string
}

// loadDataPaths loads data directory paths
func loadDataPaths() *DataPaths {
	return &DataPaths{
		Base:   DataDir,
		Lod2:   Lod2DataDir + normalizeCountryName(GetEnv("COUNTRY", "")),
		Lod3:   Lod3DataDir + normalizeCountryName(GetEnv("COUNTRY", "")),
		Tabula: TabulaDataDir,
	}
}
