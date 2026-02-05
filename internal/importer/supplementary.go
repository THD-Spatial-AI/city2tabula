package importer

import (
	"City2TABULA/internal/config"
	"City2TABULA/internal/process"
	"City2TABULA/internal/utils"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ImportSupplementaryData(conn *pgxpool.Pool, config *config.Config) error {

	// Import Tabula Data
	if err := ImportTabulaData(conn, config); err != nil {
		return err
	}

	// Import Supplementary SQL Scripts
	pipelineQueue, err := process.SupplementaryPipelineQueue(config)
	if err != nil {
		return fmt.Errorf("failed to setup DB queue: %w", err)
	}

	pipelineChan := make(chan *process.Pipeline, pipelineQueue.Len())
	// enqueue pipelines into the channel
	for !pipelineQueue.IsEmpty() {
		pipeline := pipelineQueue.Dequeue()
		if pipeline != nil {
			pipelineChan <- pipeline
		}
	}
	close(pipelineChan)

	// Actually process the pipelines with workers
	numWorkers := 1 // Use single worker for supplementary data import
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := process.NewWorker(i)
		go worker.Start(pipelineChan, conn, &wg, config)
	}

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
	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	copyCommand := fmt.Sprintf("\\copy tabula.tabula FROM '%s' DELIMITER ',' CSV HEADER", absPath)

	// Build psql command
	cmd := exec.Command("psql",
		"-h", config.DB.Host,
		"-U", config.DB.User,
		"-d", config.DB.Name,
		"-c", copyCommand)

	// Set environment variables for psql
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", config.DB.Password))

	// Capture both stdout and stderr for better debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.Error.Printf("psql command failed: %s", string(output))
		return fmt.Errorf("psql error: %v, output: %s", err, string(output))
	}

	utils.Info.Printf("psql success: %s", string(output))
	return nil
}
