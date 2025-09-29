package config

// Data directory constants
const (
	DataDir       = "data/"
	Lod2DataDir   = DataDir + "lod2/"
	Lod3DataDir   = DataDir + "lod3/"
	TabulaDataDir = DataDir + "tabula/"
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
