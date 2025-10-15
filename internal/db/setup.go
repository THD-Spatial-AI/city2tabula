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

// CreateCompleteDatabase creates the complete City2TABULA database with CityDB infrastructure
func CreateCompleteDatabase(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Creating complete City2TABULA database...")

	// Step 1: Create CityDB infrastructure
	if err := CreateCityDB(config); err != nil {
		return fmt.Errorf("failed to setup CityDB infrastructure: %w", err)
	}

	// Step 2: Create City2TABULA schemas and setup
	if err := CreateCity2TabulaSchemas(config, conn); err != nil {
		return fmt.Errorf("failed to create City2TABULA schemas: %w", err)
	}

	// Step 3: Import data
	if err := ImportAllData(config, conn); err != nil {
		return fmt.Errorf("failed to import data: %w", err)
	}

	utils.Info.Println("Complete database created successfully")
	return nil
}

// ResetCompleteDatabase completely resets everything (CityDB + City2TABULA)
func ResetCompleteDatabase(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Resetting complete database...")

	// Step 1: Drop everything
	if err := DropAllSchemas(config, conn); err != nil {
		utils.Warn.Printf("Warning during schema cleanup: %v", err)
	}

	// Step 2: Recreate everything
	if err := CreateCompleteDatabase(config, conn); err != nil {
		return fmt.Errorf("failed to recreate database: %w", err)
	}

	utils.Info.Println("Complete database reset successfully")
	return nil
}

// ResetCityDBOnly resets only the CityDB infrastructure (preserves City2TABULA schemas)
func ResetCityDBOnly(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Resetting CityDB infrastructure only...")

	// Step 1: Drop CityDB schemas
	if err := DropCityDBSchemas(config, conn); err != nil {
		utils.Warn.Printf("Warning during CityDB cleanup: %v", err)
	}

	// Step 2: Recreate CityDB
	if err := CreateCityDB(config); err != nil {
		return fmt.Errorf("failed to recreate CityDB: %w", err)
	}

	// Step 3: Re-import CityDB data only
	if err := importer.ImportCityDBData(conn, config); err != nil {
		return fmt.Errorf("failed to import CityDB data: %w", err)
	}

	utils.Info.Println("CityDB reset completed successfully")
	return nil
}

// CreateCity2TabulaSchemas creates only the City2TABULA schemas and setup
func CreateCity2TabulaSchemas(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Creating City2TABULA schemas...")

	// Create schemas
	schemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	if err := CreateSchemas(conn, schemas); err != nil {
		return fmt.Errorf("failed to create schemas: %w", err)
	}

	// Run database setup pipelines
	if err := RunDatabaseSetupPipelines(config, conn); err != nil {
		return fmt.Errorf("failed to run setup pipelines: %w", err)
	}

	utils.Info.Println("City2TABULA schemas created successfully")
	return nil
}

// RunDatabaseSetupPipelines runs the SQL setup pipelines
func RunDatabaseSetupPipelines(config *config.Config, conn *pgxpool.Pool) error {
	pipelineQueue, err := process.DBSetupPipelineQueue(config)
	if err != nil {
		return fmt.Errorf("failed to setup DB queue: %w", err)
	}

	pipelineChan := make(chan *process.Pipeline, pipelineQueue.Len())
	for !pipelineQueue.IsEmpty() {
		pipeline := pipelineQueue.Dequeue()
		if pipeline != nil {
			pipelineChan <- pipeline
		}
	}
	close(pipelineChan)

	numWorkers := config.Batch.Threads
	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := process.NewWorker(i)
		go worker.Start(pipelineChan, conn, &wg, config)
	}
	wg.Wait()

	return nil
}

// ImportAllData imports all data (supplementary + CityDB)
func ImportAllData(config *config.Config, conn *pgxpool.Pool) error {
	if err := importer.ImportSupplementaryData(conn, config); err != nil {
		return fmt.Errorf("failed to import supplementary data: %w", err)
	}
	if err := importer.ImportCityDBData(conn, config); err != nil {
		return fmt.Errorf("failed to import CityDB data: %w", err)
	}
	return nil
}

// ============================================================================
// CityDB Operations
// ============================================================================

// CreateCityDB creates CityDB core and schemas
func CreateCityDB(config *config.Config) error {
	utils.Info.Println("Setting up CityDB infrastructure...")

	// Create CityDB core
	if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.CreateDB, ""); err != nil {
		return fmt.Errorf("failed to create CityDB core: %w", err)
	}

	// Create CityDB schemas (lod2, lod3)
	schemas := []string{config.DB.Schemas.Lod2, config.DB.Schemas.Lod3}
	for _, schema := range schemas {
		if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.CreateSchema, schema); err != nil {
			return fmt.Errorf("failed to create CityDB schema %s: %w", schema, err)
		}
		utils.Info.Printf("CityDB schema %s created successfully", schema)
	}

	utils.Info.Println("CityDB infrastructure setup completed")
	return nil
}

// DropCityDBSchemas drops CityDB infrastructure schemas
func DropCityDBSchemas(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Dropping CityDB schemas...")

	// List of all CityDB schemas to drop
	cityDBSchemas := []string{
		config.DB.Schemas.Lod2,
		config.DB.Schemas.Lod3,
		config.DB.Schemas.CityDB,
		config.DB.Schemas.CityDBPkg,
	}

	// Try CityDB scripts first, then fallback to direct SQL
	schemas := []string{config.DB.Schemas.Lod2, config.DB.Schemas.Lod3}
	for _, schema := range schemas {
		if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.DropSchema, schema); err != nil {
			utils.Warn.Printf("CityDB script drop failed for %s: %v", schema, err)
		}
	}

	// Drop database using CityDB script
	if err := ExecuteCityDBScript(config, config.CityDB.SQLScripts.DropDB, ""); err != nil {
		utils.Warn.Printf("CityDB script drop database failed: %v", err)
	}

	// Force drop all CityDB schemas using direct SQL
	for _, schema := range cityDBSchemas {
		if err := DropSchemaIfExists(conn, schema); err != nil {
			utils.Warn.Printf("Warning during schema %s drop: %v", schema, err)
		}
	}

	utils.Info.Println("CityDB schemas dropped")
	return nil
}

// DropAllSchemas drops all schemas (both CityDB and City2TABULA)
func DropAllSchemas(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Dropping all schemas...")

	// Drop CityDB schemas
	if err := DropCityDBSchemas(config, conn); err != nil {
		utils.Warn.Printf("Warning during CityDB schema drop: %v", err)
	}

	// Drop City2TABULA schemas
	city2tabulaSchemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	if err := DropSchemas(conn, city2tabulaSchemas); err != nil {
		utils.Warn.Printf("Warning during City2TABULA schema drop: %v", err)
	}

	utils.Info.Println("All schemas dropped")
	return nil
}

// ============================================================================
// Utility Functions
// ============================================================================

func ExecuteCityDBScript(config *config.Config, scriptPath string, schemaName string) error {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("CityDB script not found: %s", scriptPath)
	}
	return utils.ExecuteCityDBScript(config, scriptPath, schemaName)
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

func CreateSchemaIfNotExists(conn *pgxpool.Pool, schemaName string) error {
	query := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s";`, schemaName)
	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to create schema %s: %w", schemaName, err)
	}
	utils.Info.Printf("Schema %s created successfully", schemaName)
	return nil
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

// ListCityDBSchemas lists all existing CityDB-related schemas (for debugging)
func ListCityDBSchemas(conn *pgxpool.Pool, config *config.Config) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name LIKE '%%citydb%%'
		   OR schema_name = '%s'
		   OR schema_name = '%s'
		ORDER BY schema_name;`,
		config.DB.Schemas.Lod2, config.DB.Schemas.Lod3)

	rows, err := conn.Query(context.Background(), query, config.DB.Schemas.Lod2, config.DB.Schemas.Lod3)
	if err != nil {
		return nil, fmt.Errorf("failed to list CityDB schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}
