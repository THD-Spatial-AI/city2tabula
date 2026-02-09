package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/THD-Spatial/City2TABULA/internal/config"
	"github.com/THD-Spatial/City2TABULA/internal/db"
	"github.com/THD-Spatial/City2TABULA/internal/process"
	"github.com/THD-Spatial/City2TABULA/internal/utils"
	"github.com/THD-Spatial/City2TABULA/internal/version"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Define clear and simple flags
	createDB := flag.Bool("create-db", false, "Create the complete City2TABULA database (CityDB infrastructure + schemas + data import)")
	resetAll := flag.Bool("reset-all", false, "Reset everything: drop all schemas and recreate the complete database")
	resetCityDB := flag.Bool("reset-citydb", false, "Reset only CityDB infrastructure (drop CityDB schemas, recreate them, and re-import CityDB data)")
	resetC2T := flag.Bool("reset-city2tabula", false, "Reset only City2TABULA schemas (preserve CityDB)")
	extractFeat := flag.Bool("extract-features", false, "Run the feature extraction pipeline")
	showVersion := flag.Bool("version", false, "print version and exit")
	showV := flag.Bool("v", false, "print version and exit (shorthand)")
	flag.Parse()

	// Display current version
	if *showV || *showVersion {
		fmt.Printf("%s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		os.Exit(0)
	}

	// Start timing
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		utils.Info.Println("==================================")
		utils.Info.Printf("Total runtime: %v", duration)
		utils.Info.Println("==================================")
	}()

	// Initialize logger and config
	utils.InitLogger()
	config := config.LoadConfig()

	if err := config.Validate(); err != nil {
		utils.Error.Fatal("Invalid configuration:", err)
	}

	// Connect to database
	pool, err := db.ConnectPool(&config)
	if err != nil {
		utils.Error.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.ClosePool(pool)
	utils.Info.Println("Database connection established")

	// Execute commands based on flags
	if *createDB {
		utils.Info.Println("Creating complete City2TABULA database...")
		if err := db.CreateCompleteDatabase(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to create database: %v", err)
		}
		utils.Info.Println("Database creation completed successfully")
	}

	if *resetAll {
		utils.Info.Println("Resetting complete database (everything)...")
		if err := db.ResetCompleteDatabase(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to reset complete database: %v", err)
		}
		utils.Info.Println("Database reset completed successfully.")
	}

	if *resetCityDB {
		utils.Info.Println("Resetting CityDB infrastructure only...")
		if err := db.ResetCityDBOnly(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to reset CityDB: %v", err)
		}
		utils.Info.Println("CityDB reset completed successfully")
	}

	if *resetC2T {
		utils.Info.Println("Resetting City2TABULA schemas only...")
		// Drop City2TABULA schemas
		c2tSchemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
		for _, schema := range c2tSchemas {
			if err := db.DropSchemaIfExists(pool, schema); err != nil {
				utils.Warn.Printf("Warning dropping schema %s: %v", schema, err)
			}
		}
		db.RunCity2TabulaDBSetup(&config, pool)
		utils.Info.Println("City2TABULA schemas reset completed successfully")
	}

	if *extractFeat {
		utils.Info.Println("Running feature extraction pipeline...")
		if err := runFeatureExtraction(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to run feature extraction: %v", err)
		}
		utils.Info.Println("Feature extraction completed successfully")
	}
}

// runFeatureExtraction handles the feature extraction pipeline
func runFeatureExtraction(config *config.Config, pool *pgxpool.Pool) error {
	// Get building IDs from CityDB
	lod2BldIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod2)
	if err != nil {
		return fmt.Errorf("failed to get LOD2 building IDs: %w", err)
	}

	if len(lod2BldIDs) == 0 {
		utils.Warn.Println("No LOD2 buildings found in CityDB. Skipping LOD2 feature extraction.")
	} else {
		utils.Info.Printf("Found %d buildings for LOD2 in CityDB", len(lod2BldIDs))
	}

	lod3BldIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod3)
	if err != nil {
		return fmt.Errorf("failed to get LOD3 building IDs: %w", err)
	}

	if len(lod3BldIDs) == 0 {
		utils.Warn.Println("No LOD3 buildings found in CityDB. Skipping LOD3 feature extraction.")
	} else {
		utils.Info.Printf("Found %d buildings for LOD3 in CityDB", len(lod3BldIDs))
	}

	// Check if there are any buildings to process
	totalBuildings := len(lod2BldIDs) + len(lod3BldIDs)
	if totalBuildings == 0 {
		utils.Warn.Println("No buildings found in either LOD2 or LOD3 schemas. Nothing to extract.")
		return nil
	}

	// Create batches
	batchesLOD2 := utils.CreateBatches(lod2BldIDs, config.Batch.Size)
	batchesLOD3 := utils.CreateBatches(lod3BldIDs, config.Batch.Size)

	if batchesLOD2 != nil {
		utils.Debug.Printf("Created %d batches for LOD2", len(batchesLOD2))
	}
	if batchesLOD3 != nil {
		utils.Debug.Printf("Created %d batches for LOD3", len(batchesLOD3))
	}

	// Build feature extraction queue
	pipQueue, err := process.BuildFeatureExtractionQueue(config, batchesLOD2, batchesLOD3)
	if err != nil {
		return fmt.Errorf("failed to build feature extraction queue: %w", err)
	}

	if pipQueue.Len() > 0 {
		utils.PrintPipelineQueueInfo(pipQueue.Len(), len(pipQueue.Peek().Jobs))
	} else {
		utils.Warn.Printf("Pipeline queue is empty - this shouldn't happen if buildings were found. Check batch creation logic.")
		// Continue anyway - workers will just have no work to do
	}

	// Create pipeline channel
	pipChan := make(chan *process.Pipeline, pipQueue.Len())
	for !pipQueue.IsEmpty() {
		pipeline := pipQueue.Dequeue()
		if pipeline != nil {
			pipChan <- pipeline
		}
	}
	close(pipChan)

	// Start workers
	numWorkers := config.Batch.Threads
	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := process.NewWorker(i)
		go worker.Start(pipChan, pool, &wg, config)
	}
	wg.Wait()

	return nil
}
