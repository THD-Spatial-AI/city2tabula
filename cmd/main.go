package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"City2TABULA/internal/config"
	"City2TABULA/internal/db"
	"City2TABULA/internal/process"
	"City2TABULA/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Define clear and simple flags
	createDB := flag.Bool("create_db", false, "Create the complete City2TABULA database (CityDB infrastructure + schemas + data import)")
	resetAll := flag.Bool("reset_all", false, "Reset everything: drop all schemas and recreate the complete database")
	resetCityDB := flag.Bool("reset_citydb", false, "Reset only CityDB infrastructure (drop CityDB schemas, recreate them, and re-import CityDB data)")
	resetCity2Tabula := flag.Bool("reset_city2tabula", false, "Reset only City2TABULA schemas (preserve CityDB)")
	extractFeatures := flag.Bool("extract_features", false, "Run the feature extraction pipeline")

	flag.Parse()

	// Start timing
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		utils.Info.Printf("Total runtime: %v", duration)
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
	defer pool.Close()
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
		utils.Info.Println("Complete database reset completed successfully")
	}

	if *resetCityDB {
		utils.Info.Println("Resetting CityDB infrastructure only...")
		if err := db.ResetCityDBOnly(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to reset CityDB: %v", err)
		}
		utils.Info.Println("CityDB reset completed successfully")
	}

	if *resetCity2Tabula {
		utils.Info.Println("Resetting City2TABULA schemas only...")
		// Drop City2TABULA schemas
		city2tabulaSchemas := []string{config.DB.Schemas.City2Tabula, config.DB.Schemas.Tabula}
		for _, schema := range city2tabulaSchemas {
			if err := db.DropSchemaIfExists(pool, schema); err != nil {
				utils.Warn.Printf("Warning dropping schema %s: %v", schema, err)
			}
		}
		// Recreate City2TABULA schemas
		if err := db.CreateCity2TabulaSchemas(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to recreate City2TABULA schemas: %v", err)
		}
		// Import supplementary data
		if err := db.ImportAllData(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to import data: %v", err)
		}
		utils.Info.Println("City2TABULA schemas reset completed successfully")
	}

	if *extractFeatures {
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
	lod2BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod2)
	if err != nil {
		return fmt.Errorf("failed to get LOD2 building IDs: %w", err)
	}

	if len(lod2BuildingIDs) == 0 {
		utils.Warn.Println("No LOD2 buildings found in CityDB. Skipping LOD2 feature extraction.")
	} else {
		utils.Info.Printf("Found %d buildings for LOD2 in CityDB", len(lod2BuildingIDs))
	}

	lod3BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod3)
	if err != nil {
		return fmt.Errorf("failed to get LOD3 building IDs: %w", err)
	}

	if len(lod3BuildingIDs) == 0 {
		utils.Warn.Println("No LOD3 buildings found in CityDB. Skipping LOD3 feature extraction.")
	} else {
		utils.Info.Printf("Found %d buildings for LOD3 in CityDB", len(lod3BuildingIDs))
	}

	// Check if there are any buildings to process
	totalBuildings := len(lod2BuildingIDs) + len(lod3BuildingIDs)
	if totalBuildings == 0 {
		utils.Warn.Println("No buildings found in either LOD2 or LOD3 schemas. Nothing to extract.")
		return nil
	}

	// Create batches
	batchesLOD2 := utils.CreateBatches(lod2BuildingIDs, config.Batch.Size)
	batchesLOD3 := utils.CreateBatches(lod3BuildingIDs, config.Batch.Size)

	if batchesLOD2 != nil {
		utils.Debug.Printf("Created %d batches for LOD2", len(batchesLOD2))
	}
	if batchesLOD3 != nil {
		utils.Debug.Printf("Created %d batches for LOD3", len(batchesLOD3))
	}

	// Build feature extraction queue
	pipelineQueue, err := process.BuildFeatureExtractionQueue(config, batchesLOD2, batchesLOD3)
	if err != nil {
		return fmt.Errorf("failed to build feature extraction queue: %w", err)
	}

	if pipelineQueue.Len() > 0 {
		utils.PrintPipelineQueueInfo(pipelineQueue.Len(), len(pipelineQueue.Peek().Jobs))
	} else {
		utils.Warn.Printf("Pipeline queue is empty - this shouldn't happen if buildings were found. Check batch creation logic.")
		// Continue anyway - workers will just have no work to do
	}

	// Create pipeline channel
	pipelineChan := make(chan *process.Pipeline, pipelineQueue.Len())
	for !pipelineQueue.IsEmpty() {
		pipeline := pipelineQueue.Dequeue()
		if pipeline != nil {
			pipelineChan <- pipeline
		}
	}
	close(pipelineChan)

	// Start workers
	numWorkers := config.Batch.Threads
	var wg sync.WaitGroup
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := process.NewWorker(i)
		go worker.Start(pipelineChan, pool, &wg, config)
	}
	wg.Wait()

	return nil
}
