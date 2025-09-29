package db

import (
	"City2TABULA/internal/config"
	"City2TABULA/internal/importer"
	"City2TABULA/internal/process"
	"City2TABULA/internal/utils"
	"context"
	"fmt"
	"os"

	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupOperation defines database setup operations
type SetupOperation struct {
	Name     string
	SQLPath  string
	Priority int
}

// Create3City2TabulaDB creates the 3D to Tabula database tables
func CreateCity2TABULADB(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Creating 3D to Tabula database tables...")

	// Create schemas
	schemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	if err := CreateSchemas(conn, schemas); err != nil {
		return fmt.Errorf("failed to create schemas: %w", err)
	}

	pipelineQueue, err := process.DBSetupPipelineQueue(config)
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

	// start workers
	numWorkers := config.Batch.Threads // or runtime.NumCPU()
	var wg sync.WaitGroup
	utils.Info.Println("Database connection established")
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := process.NewWorker(i)
		go worker.Start(pipelineChan, conn, &wg, config)
	}
	wg.Wait()

	if err := importer.ImportSupplementaryData(conn, config); err != nil {
		return fmt.Errorf("failed to import supplementary data: %w", err)
	}
	if err := importer.ImportCityDBData(conn, config); err != nil {
		return fmt.Errorf("failed to import CityDB data: %w", err)
	}
	utils.Info.Println("City2TABULA database created successfully")
	return nil
}

// Reset3DCityToTabulaDB performs training database reset
func Reset3DCityToTabulaDB(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Starting training database reset...")

	if err := Drop3DCityToTabulaDB(conn, config); err != nil {
		return fmt.Errorf("failed to drop training database: %w", err)
	}

	if err := CreateCity2TABULADB(config, conn); err != nil {
		return fmt.Errorf("failed to recreate training database: %w", err)
	}
	if err := importer.ImportSupplementaryData(conn, config); err != nil {
		return fmt.Errorf("failed to import supplementary data: %w", err)
	}
	if err := importer.ImportCityDBData(conn, config); err != nil {
		return fmt.Errorf("failed to import CityDB data: %w", err)
	}
	utils.Info.Println("Training database reset completed successfully")
	return nil
}

// Drop3DCityToTabulaDB drops all training-related schemas
func Drop3DCityToTabulaDB(conn *pgxpool.Pool, config *config.Config) error {
	schemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	return DropSchemas(conn, schemas)
}

// CityDB Operations
func ResetCityDB(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Starting CityDB reset process...")

	if err := DropCityDB(config, conn); err != nil {
		utils.Warn.Printf("Warning during CityDB drop: %v", err)
	}

	if err := CreateCityDB(config); err != nil {
		return fmt.Errorf("failed to setup CityDB: %w", err)
	}

	utils.Info.Println("CityDB reset completed successfully")
	return nil
}

// createCityDB creates CityDB core and schemas
func CreateCityDB(config *config.Config) error {
	utils.Info.Println("Setting up CityDB...")

	// Create CityDB core
	if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.CreateDB, ""); err != nil {
		return fmt.Errorf("failed to create CityDB core: %w", err)
	}

	// Create CityDB schemas
	schemas := []string{config.DB.Schemas.Lod2, config.DB.Schemas.Lod3}
	for _, schema := range schemas {
		if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.CreateSchema, schema); err != nil {
			return fmt.Errorf("failed to create CityDB schema %s: %w", schema, err)
		}
		utils.Info.Printf("CityDB schema %s created successfully", schema)
	}

	utils.Info.Println("CityDB setup completed successfully")
	return nil
}

// DropCityDB drops CityDB infrastructure
func DropCityDB(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Dropping CityDB...")

	// Drop schemas first
	schemas := []string{config.DB.Schemas.Lod2, config.DB.Schemas.Lod3}
	for _, schema := range schemas {
		if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.DropSchema, schema); err != nil {
			utils.Warn.Printf("Warning during schema %s drop: %v", schema, err)
		}
	}

	// Drop database
	if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.DropDB, ""); err != nil {
		utils.Warn.Printf("Warning during database drop: %v", err)
	}

	utils.Info.Println("CityDB dropped successfully")
	return nil
}

func CreateSchemas(conn *pgxpool.Pool, schemas []string) error {
	for _, schema := range schemas {
		if err := CreateSchemaIfNotExists(conn, schema); err != nil {
			return fmt.Errorf("failed to create schema %s: %w", schema, err)
		}
	}
	return nil
}

func DropSchemas(conn *pgxpool.Pool, schemas []string) error {
	for _, schema := range schemas {
		if err := DropSchemaIfExists(conn, schema); err != nil {
			return fmt.Errorf("failed to drop schema %s: %w", schema, err)
		}
	}
	return nil
}

func DropAllSchemas(conn *pgxpool.Pool, config *config.Config) error {
	for _, schema := range config.DB.Schemas.All() {
		if err := DropSchemaIfExists(conn, schema); err != nil {
			utils.Warn.Printf("Warning during schema %s drop: %v", schema, err)
		}
	}
	return nil
}

func ExecuteCityDBScript(config *config.Config, scriptPath string, schemaName string) error {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("CityDB script not found: %s", scriptPath)
	}
	return utils.ExecuteCityDBScript(config, scriptPath, schemaName)
}

// Schema utility functions (kept as-is for compatibility)
func CreateSchemaIfNotExists(conn *pgxpool.Pool, schemaName string) error {
	query := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s";`, schemaName)
	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to create schema %s: %w", schemaName, err)
	}
	utils.Info.Printf("Schema %s created successfully", schemaName)
	return nil
}

func SchemaExists(conn *pgxpool.Pool, schemaName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1);`
	if err := conn.QueryRow(context.Background(), query, schemaName).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check if schema %s exists: %w", schemaName, err)
	}
	return exists, nil
}

func DropSchemaIfExists(conn *pgxpool.Pool, schemaName string) error {
	query := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schemaName)
	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to drop schema %s: %w", schemaName, err)
	}
	utils.Info.Printf("Schema %s dropped successfully", schemaName)
	return nil
}
