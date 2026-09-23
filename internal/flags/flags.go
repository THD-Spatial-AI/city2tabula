package flags

import "flag"

type Flags struct {
	CreateDB        bool
	ResetDB         bool
	ImportData      bool
	ResetC2T        bool
	ExtractFeatures bool
	LinkPylovo      bool
	ShowVersion     bool
	ShowV           bool
	Bbox            string
	BboxMode        string
}

func ParseFlags() *Flags {
	f := &Flags{}
	flag.BoolVar(&f.CreateDB, "create-db", false, "Create the complete City2TABULA database (CityDB infrastructure + schemas + first data import). Refuses to run if the database already exists; use -import-data to add data to one")
	flag.BoolVar(&f.ResetDB, "reset-db", false, "Destructive. Drop all schemas, including every extracted feature and hand correction, and recreate the database from scratch")
	flag.BoolVar(&f.ImportData, "import-data", false, "Import new 3D city data into an existing database, skipping files already imported. Follow with -extract-features to process the new buildings")
	flag.BoolVar(&f.ResetC2T, "reset-city2tabula", false, "Reset only City2TABULA schemas (preserve CityDB)")
	flag.BoolVar(&f.ExtractFeatures, "extract-features", false, "Run the feature extraction pipeline over buildings not yet processed. Safe to re-run; already-processed buildings are skipped")
	flag.BoolVar(&f.LinkPylovo, "link-pylovo", false, "Link 3D buildings to PyLovo res/oth via IoU spatial join (requires -extract-features to have run first)")
	flag.BoolVar(&f.ShowVersion, "version", false, "print version and exit")
	flag.BoolVar(&f.ShowV, "v", false, "print version and exit (shorthand)")
	flag.StringVar(&f.Bbox, "bbox", "", "Bounding box spatial filter for -import-data, format xmin,ymin,xmax,ymax[,srid] (passed through to citydb-tool)")
	flag.StringVar(&f.BboxMode, "bbox-mode", "intersects", "Bounding box filter mode for -bbox: intersects, contains, or on_tile")
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
type ResetC2TMsg Msg
type ExtractFeaturesMsg Msg
type LinkPylovoMsg Msg
type ImportDataMsg Msg

// Define messages for each flag
var (
	CreateDBMessages = CreateDBMsg{
		Custom: `Database already exists.

		Pick the command that matches what you are trying to do:

		----------------------------

		Adding new 3D city data to this database

			c2t -import-data
			c2t -extract-features

		Files already imported are skipped and buildings already processed are
		not reprocessed, so both are safe to re-run. This keeps everything
		already extracted, including any hand-corrected buildings.

		----------------------------

		Building a second database alongside this one

		Change DB_NAME, then re-run -create-db:

			Linux:      make configure
			Windows:    setup.bat configure
			PowerShell: .\setup.ps1 configure

		----------------------------

		Starting over and discarding everything in this database

			Linux:      make reset-db
			Windows:    setup.bat reset-db
			PowerShell: .\setup.ps1 reset-db

		----------------------------
		`,
		Progress: "Creating the database...",
		Success:  "Database created successfully",
		Error:    "Failed to create database",
	}
	ResetDBMessages = ResetDBMsg{
		Progress: "Resetting the database...",
		Success:  "Database reset successfully",
		Error:    "Failed to reset database",
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
	LinkPylovoMessages = LinkPylovoMsg{
		Progress: "Building OSM link table...",
		Success:  "OSM link table built successfully",
		Error:    "Failed to build OSM link table",
	}
	ImportDataMessages = ImportDataMsg{
		Progress: "Importing data into existing CityDB schemas...",
		Success:  "Data imported successfully",
		Error:    "Failed to import data",
	}
)

// Define a struct to hold all messages for easy access
type Messages struct {
	CreateDB        CreateDBMsg
	ResetDB         ResetDBMsg
	ResetC2T        ResetC2TMsg
	ExtractFeatures ExtractFeaturesMsg
	LinkPylovo      LinkPylovoMsg
	ImportData      ImportDataMsg
}

var AllMessages = Messages{
	CreateDB:        CreateDBMessages,
	ResetDB:         ResetDBMessages,
	ResetC2T:        ResetC2TMessages,
	ExtractFeatures: ExtractFeaturesMessages,
	LinkPylovo:      LinkPylovoMessages,
	ImportData:      ImportDataMessages,
}
