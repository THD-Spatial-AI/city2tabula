package importer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ImportSupplementaryData(conn *pgxpool.Pool, config *config.Config) error {

	// Import Tabula Data
	if err := ImportTabulaData(conn, config); err != nil {
		return err
	}

	// Import Supplementary SQL Scripts
	jobQueue, err := process.SupplementaryJobQueue(config)
	if err != nil {
		return fmt.Errorf("failed to setup DB queue: %w", err)
	}

	// Supplementary scripts must run in order, so we use a single worker here.
	jobChan := jobQueue.ToChannel()
	var wg sync.WaitGroup
	wg.Add(1)
	go process.NewWorker(1).Start(jobChan, conn, &wg, config)
	wg.Wait()

	utils.Info.Println("Supplementary data imported successfully")
	return nil
}

// ImportTabulaData orchestrates the import of Tabula data into the database
func ImportTabulaData(conn *pgxpool.Pool, config *config.Config) error {
	csvFilePath := config.Data.Tabula + config.Country + ".csv"

	utils.Info.Printf("Importing Tabula data from %s", csvFilePath)

	if err := ImportCsvWithPsql(csvFilePath, config); err != nil {
		return fmt.Errorf("failed to import Tabula data: %w", err)
	}
	utils.Info.Printf("Tabula data imported from %s", csvFilePath)
	return nil
}

func ImportCsvWithPsql(filePath string, config *config.Config) error {
	cmd, err := getCsvImportCommand(filePath, config)
	if err != nil {
		return err
	}

	// Capture both stdout and stderr for better debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("psql command failed: %s", string(output))
		return fmt.Errorf("psql error: %v, output: %s", err, string(output))
	}

	utils.Info.Printf("psql success: %s", string(output))
	return nil
}

// getCsvImportCommand builds the psql \copy command that loads filePath into
// tabula.tabula. Command construction is separated from execution so tests can
// assert on the resulting *exec.Cmd (args, env) without actually running psql.
func getCsvImportCommand(filePath string, config *config.Config) (*exec.Cmd, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	copyCommand := fmt.Sprintf("\\copy tabula.tabula FROM '%s' DELIMITER ',' CSV HEADER", absPath)

	cmd := exec.Command("psql",
		"-h", config.DB.Host,
		"-p", config.DB.Port,
		"-U", config.DB.User,
		"-d", config.DB.Name,
		"-c", copyCommand)

	// Inherit the current environment (PATH, HOME, locale, ...) rather than
	// replacing it outright - psql needs more than just PGPASSWORD to run correctly.
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", config.DB.Password))

	return cmd, nil
}
