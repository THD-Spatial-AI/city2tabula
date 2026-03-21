package db

import (
	"context"
	"fmt"
	"os"

	"github.com/THD-Spatial/City2TABULA/internal/config"
	"github.com/THD-Spatial/City2TABULA/internal/importer"
	"github.com/THD-Spatial/City2TABULA/internal/process"
	"github.com/THD-Spatial/City2TABULA/internal/utils"

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

	// Step 1: Create CityDB infrastructure
	if err := CreateCityDB(config); err != nil {
		return fmt.Errorf("failed to setup CityDB infrastructure: %w", err)
	}

	// Step 2: Create City2TABULA schemas and setup
	if err := RunCity2TabulaDBSetup(config, conn); err != nil {
		return fmt.Errorf("failed to create City2TABULA schemas: %w", err)
	}

	// Step 3: Import data
	if err := ImportAllData(config, conn); err != nil {
		return fmt.Errorf("failed to import data: %w", err)
	}

	return nil
}

// ResetCompleteDatabase completely resets everything (CityDB + City2TABULA)
func ResetCompleteDatabase(config *config.Config, conn *pgxpool.Pool) error {

	// Step 1: Drop everything
	if err := DropAllSchemas(config, conn); err != nil {
		utils.Warn.Printf("Warning during schema cleanup: %v", err)
	}

	// Step 2: Recreate everything
	if err := CreateCompleteDatabase(config, conn); err != nil {
		return fmt.Errorf("failed to recreate database: %w", err)
	}

	utils.Debug.Println("Complete database reset successfully")
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

// RunCity2TabulaDBSetup creates the city2tabula and tabula schemas and runs all setup job queues.
func RunCity2TabulaDBSetup(config *config.Config, conn *pgxpool.Pool) error {
	schemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	if err := CreateSchemas(conn, schemas); err != nil {
		return fmt.Errorf("failed to create schemas: %w", err)
	}
	if err := setupMainDB(config, conn); err != nil {
		return err
	}
	return setupSupplementaryDB(config, conn)
}

// ResetCity2TabulaSchemas drops the city2tabula and tabula schemas and rebuilds them from scratch.
// Use this when you want to re-run the City2TABULA setup without touching CityDB.
func ResetCity2TabulaSchemas(config *config.Config, conn *pgxpool.Pool) error {
	schemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	for _, schema := range schemas {
		if err := DropSchemaIfExists(conn, schema); err != nil {
			utils.Warn.Printf("Warning dropping schema %s: %v", schema, err)
		}
	}
	return RunCity2TabulaDBSetup(config, conn)
}

// setupMainDB runs the main DB setup job queue: PostgreSQL functions and main table schemas.
func setupMainDB(config *config.Config, conn *pgxpool.Pool) error {
	queue, err := process.MainDBSetupJobQueue(config)
	if err != nil {
		return fmt.Errorf("failed to build main DB setup queue: %w", err)
	}
	if err := process.RunJobQueue(queue, conn, config); err != nil {
		return fmt.Errorf("main DB setup failed: %w", err)
	}
	utils.Info.Println("Main database setup completed")
	return nil
}

// setupSupplementaryDB runs the supplementary DB setup job queue: tabula classification table schemas.
func setupSupplementaryDB(config *config.Config, conn *pgxpool.Pool) error {
	queue, err := process.SupplementaryDBSetupJobQueue(config)
	if err != nil {
		return fmt.Errorf("failed to build supplementary DB setup queue: %w", err)
	}
	if err := process.RunJobQueue(queue, conn, config); err != nil {
		return fmt.Errorf("supplementary DB setup failed: %w", err)
	}
	utils.Info.Println("Supplementary database setup completed")
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

// DropAllSchemas drops all schemas (both CityDB and City2TABULA)
func DropAllSchemas(config *config.Config, conn *pgxpool.Pool) error {
	utils.Info.Println("Dropping all schemas...")

	// Drop CityDB schemas
	if err := DropCityDBSchemas(config, conn); err != nil {
		utils.Warn.Printf("Warning during CityDB schema drop: %v", err)
	}

	// Drop City2TABULA schemas
	city2tabulaSchemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
	if err := DropCity2TabulaSchemas(conn, city2tabulaSchemas); err != nil {
		utils.Warn.Printf("Warning during City2TABULA schema drop: %v", err)
	}

	utils.Info.Println("All schemas dropped")
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

	// Force drop all CityDB schemas using direct SQL
	for _, schema := range cityDBSchemas {
		if err := DropSchemaIfExists(conn, schema); err != nil {
			utils.Warn.Printf("Warning during schema %s drop: %v", schema, err)
		}
	}

	utils.Debug.Println("CityDB schemas dropped")
	return nil
}

// ============================================================================
// Utility Functions
// ============================================================================

// CreateCityDB creates CityDB core and schemas
func CreateCityDB(config *config.Config) error {
	utils.Debug.Println("Setting up CityDB infrastructure...")

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
		utils.Debug.Printf("CityDB schema %s created successfully", schema)
	}

	utils.Debug.Println("CityDB infrastructure setup completed")
	return nil
}

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

func DropCity2TabulaSchemas(conn *pgxpool.Pool, schemas []string) error {
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
	utils.Debug.Printf("Schema %s created successfully", schemaName)
	return nil
}

func DropSchemaIfExists(conn *pgxpool.Pool, schemaName string) error {
	query := fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE;`, schemaName)
	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to drop schema %s: %w", schemaName, err)
	}
	utils.Debug.Printf("Schema %s dropped successfully", schemaName)
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
