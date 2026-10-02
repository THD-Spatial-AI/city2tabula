package importer

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportCityDBData orchestrates the import of CityDB data into the database.
// bbox/bboxMode are an optional spatial filter passed straight through to
// citydb-tool (xmin,ymin,xmax,ymax[,srid]) — pass "" for the existing
// whole-directory behaviour.
func ImportCityDBData(conn *pgxpool.Pool, config *config.Config, bbox, bboxMode string) error {

	// Construct the path to the CityDB executable
	cityDBExecPath := path.Join(config.CityDB.ToolPath, "citydb")

	// Returned rather than fatal: this runs inside the HTTP server's run
	// goroutine as well as the CLI, and exiting the process would take the
	// server down over one misconfigured run.
	if _, err := os.Stat(cityDBExecPath); os.IsNotExist(err) {
		return fmt.Errorf("citydb executable not found at %s (set CITYDB_TOOL_PATH to the citydb-tool directory)", cityDBExecPath)
	}

	// Test the citydb connection using the -help flag
	if err := testCityDBExecPath(cityDBExecPath); err != nil {
		return err
	}

	// Every folder is validated before the first import, so a bad attribution
	// file stops the run before any data lands.
	datasets, err := DiscoverDatasets(config)
	if err != nil {
		return err
	}
	if len(datasets) == 0 {
		utils.Warn.Printf("No dataset folders under %s or %s, nothing to import", config.Data.Lod2, config.Data.Lod3)
	}
	for _, d := range datasets {
		if err := importCityDBFiles(cityDBExecPath, d, config, bbox, bboxMode); err != nil {
			return err
		}
	}
	return nil
}

func testCityDBExecPath(cityDBExecPath string) error {
	cmd := exec.Command(cityDBExecPath, "-help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("CityDB connection test failed: %s", string(output))
		return err
	}
	return nil
}

// importCityDBFiles imports each of a dataset's model folders with its own
// format into the dataset's schema, tagging every feature with the dataset_id
// as its lineage.
func importCityDBFiles(cityDBExecPath string, d Dataset, config *config.Config, bbox, bboxMode string) error {
	for _, in := range d.Inputs {
		cmd := getCityDBImportCommand(cityDBExecPath, in.path, d.Schema, in.format, d.Attribution.DatasetID, config, bbox, bboxMode)
		if err := executeCityDBCommand(cmd, fmt.Sprintf("%s %s", d.Attribution.DatasetID, in.label)); err != nil {
			return err
		}
	}

	utils.Info.Printf("%s imported from %s into %s", d.Attribution.DatasetID, d.Dir, d.Schema)
	return nil
}

// executeCityDBCommand executes a CityDB command with proper logging
func executeCityDBCommand(cmd *exec.Cmd, description string) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("%s import command failed: %v\nOutput: %s", description, err, string(output))
		return err
	}

	utils.Info.Printf("%s import completed successfully", description)
	return nil
}

// getCityDBImportCommand creates a CityDB import command for the specified format.
// bbox/bboxMode add citydb-tool's own spatial filter (-b/--bbox-mode) when bbox
// is non-empty; pass "" to import the whole directory as before. lineage is
// stored on every imported feature (3DCityDB feature.lineage), which script 04
// reads as the building's dataset_id.
// dataPath is the folder citydb-tool reads.
func getCityDBImportCommand(cityDBExecPath, dataPath, dbSchema, format, lineage string, config *config.Config, bbox, bboxMode string) *exec.Cmd {
	args := []string{
		"import",
		"--log-level=debug",
		format,               // "citygml" or "cityjson"
		"--import-mode=skip", // Skip existing data
		fmt.Sprintf("--threads=%d", config.Batch.Threads),
		fmt.Sprintf("--db-name=%s", config.DB.Name),
		fmt.Sprintf("--db-user=%s", config.DB.User),
		fmt.Sprintf("--db-password=%s", config.DB.Password),
		fmt.Sprintf("--db-host=%s", config.DB.Host),
		fmt.Sprintf("--db-port=%s", config.DB.Port),
		fmt.Sprintf("--db-schema=%s", dbSchema),
		fmt.Sprintf("--lineage=%s", lineage),
	}

	if config.CityDB.ImportLimit > 0 {
		args = append(args, fmt.Sprintf("--limit=%d", config.CityDB.ImportLimit))
	}

	if bbox != "" {
		args = append(args, fmt.Sprintf("--bbox=%s", bbox), fmt.Sprintf("--bbox-mode=%s", bboxMode))
	}

	args = append(args, dataPath)
	return exec.Command(cityDBExecPath, args...)
}
