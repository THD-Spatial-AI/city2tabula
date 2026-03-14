package flags

import "flag"

type Flags struct {
	CreateDB        bool
	ResetDB         bool
	ResetCityDB     bool
	ResetC2T        bool
	ExtractFeatures bool
	ShowVersion     bool
	ShowV           bool
}

func ParseFlags() *Flags {
	f := &Flags{}
	flag.BoolVar(&f.CreateDB, "create-db", false, "Create the complete City2TABULA database (CityDB infrastructure + schemas + data import)")
	flag.BoolVar(&f.ResetDB, "reset-db", false, "Reset everything: drop all schemas and recreate the complete database")
	flag.BoolVar(&f.ResetCityDB, "reset-citydb", false, "Reset only CityDB infrastructure (drop CityDB schemas, recreate them, and re-import CityDB data)")
	// flag.BoolVar(&f.ImportData, "import-data", false, "Import data into existing CityDB schemas (useful if you want to keep existing City2TABULA schemas and import new 3D city data)")
	flag.BoolVar(&f.ResetC2T, "reset-city2tabula", false, "Reset only City2TABULA schemas (preserve CityDB)")
	flag.BoolVar(&f.ExtractFeatures, "extract-features", false, "Run the feature extraction pipeline")
	flag.BoolVar(&f.ShowVersion, "version", false, "print version and exit")
	flag.BoolVar(&f.ShowV, "v", false, "print version and exit (shorthand)")
	flag.Parse()
	return f
}

type Msg struct {
	Custom   string
	Progress string
	Success  string
	Error    string
}

type CreateDBMsg Msg
type ResetDBMsg Msg
type ResetCityDBMsg Msg
type ResetC2TMsg Msg
type ExtractFeaturesMsg Msg
type ImportDataMsg Msg

// Define messages for each flag
var (
	CreateDBMessages = CreateDBMsg{
		Custom:   "Database already exists!",
		Progress: "Creating the complete database...",
		Success:  "Database created successfully",
		Error:    "Failed to create database",
	}
	ResetDBMessages = ResetDBMsg{
		Custom: `To reset the database, use ONE of the following commands based on your operating system:

		----------------------------

		1) For Linux: make reset-db

		2) For Windows: setup.bat reset-db

		3) For PowerShell: .\setup.ps1 reset-db

		----------------------------
		`,
		Progress: "Resetting the database...",
		Success:  "Database reset successfully",
		Error:    "Failed to reset database",
	}
	ResetCityDBMessages = ResetCityDBMsg{
		Progress: "Resetting CityDB...",
		Success:  "CityDB reset successfully",
		Error:    "Failed to reset CityDB",
	}
	ResetC2TMessages = ResetC2TMsg{
		Progress: "Resetting City2TABULA schemas...",
		Success:  "City2TABULA schemas reset successfully",
		Error:    "Failed to reset City2TABULA schemas",
	}
	ExtractFeaturesMessages = ExtractFeaturesMsg{
		Progress: "Extracting features...",
		Success:  "Feature extraction completed successfully",
		Error:    "Failed to extract features",
	}
)

// Define a struct to hold all messages for easy access
type Messages struct {
	CreateDB        CreateDBMsg
	ResetDB         ResetDBMsg
	ResetCityDB     ResetCityDBMsg
	ResetC2T        ResetC2TMsg
	ExtractFeatures ExtractFeaturesMsg
}

var AllMessages = Messages{
	CreateDB:        CreateDBMessages,
	ResetDB:         ResetDBMessages,
	ResetCityDB:     ResetCityDBMessages,
	ResetC2T:        ResetC2TMessages,
	ExtractFeatures: ExtractFeaturesMessages,
}
