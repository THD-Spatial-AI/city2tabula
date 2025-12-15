package importer

import (
	"City2TABULA/internal/config"
	"City2TABULA/internal/utils"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportCityDBData orchestrates the import of CityDB data into the database
func ImportCityDBData(conn *pgxpool.Pool, config *config.Config) error {

	// Construct the path to the CityDB executable
	cityDBExecutable := path.Join(config.CityDB.ToolPath, "citydb")

	// Check if the CityDB executable exists
	if _, err := os.Stat(cityDBExecutable); os.IsNotExist(err) {
		utils.Error.Fatalf("CityDB executable not found at %s", cityDBExecutable)
		return err
	}

	// Test the citydb connection using the -help flag
	if err := testCityDBExecutable(cityDBExecutable); err != nil {
		return err
	}

	// Import LOD2 data (both CityGML and CityJSON formats)
	if err := importCityDBFiles(cityDBExecutable, config.Data.Lod2, config.DB.Schemas.Lod2, "LOD2", config); err != nil {
		return err
	}

	// Import LOD3 data (both CityGML and CityJSON formats)
	if err := importCityDBFiles(cityDBExecutable, config.Data.Lod3, config.DB.Schemas.Lod3, "LOD3", config); err != nil {
		return err
	}
	return nil
}

func testCityDBExecutable(cityDBExecutable string) error {
	cmd := exec.Command(cityDBExecutable, "-help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("CityDB connection test failed: %s", string(output))
		return err
	}
	return nil
}

// importCityDBFiles imports both CityGML and CityJSON files from a directory
func importCityDBFiles(cityDBExecutable, dataPath, dbSchema, lodLevel string, config *config.Config) error {
	// Import CityGML files
	cmd := getCityDBImportCommand(cityDBExecutable, dataPath, dbSchema, "citygml", config)
	if cmd == nil {
		return fmt.Errorf("no CityGML files found in %s for %s", dataPath, lodLevel)
	}
	if err := executeCityDBCommand(cmd, fmt.Sprintf("%s CityGML", lodLevel)); err != nil {
		return err
	}

	// Import CityJSON files
	cmd = getCityDBImportCommand(cityDBExecutable, dataPath, dbSchema, "cityjson", config)
	if cmd == nil {
		return fmt.Errorf("no CityJSON files found in %s for %s", dataPath, lodLevel)
	}
	if err := executeCityDBCommand(cmd, fmt.Sprintf("%s CityJSON", lodLevel)); err != nil {
		return err
	}

	utils.Info.Printf("%s data imported successfully", lodLevel)
	return nil
}

// executeCityDBCommand executes a CityDB command with proper logging
func executeCityDBCommand(cmd *exec.Cmd, description string) error {
	utils.Info.Printf("Executing %s import command: %s", description, cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("%s import command failed: %v\nOutput: %s", description, err, string(output))
		return err
	}

	utils.Info.Printf("%s import completed successfully", description)
	return nil
}

// getCityDBImportCommand creates a CityDB import command for the specified format
func getCityDBImportCommand(cityDBExecutable, dataPath, dbSchema, format string, config *config.Config) *exec.Cmd {
	// Check file path exists before creating command
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		utils.Error.Fatalf("Data path not found: %s", dataPath)
		return nil
	}

	return exec.Command(cityDBExecutable,
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
		dataPath)
}
