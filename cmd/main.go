package main

import (
	"flag"
	"sync"
	"time"

	"City2TABULA/internal/config"
	"City2TABULA/internal/db"
	"City2TABULA/internal/process"
	"City2TABULA/internal/utils"
)

func main() {
	// Define flags
	createDB := flag.Bool("create_db", false, "Create the City2TABULA database and CityDB schemas required to store the 3D city models and import the data")
	resetDB := flag.Bool("reset_db", false, "Reset the City2TABULA database and CityDB schemas (drops all tables and re-creates them)")
	resetCity2Tabula := flag.Bool("reset_City2TABULA", false, "Reset only the City2TABULA database (drops all tables and re-creates them)")
	extractFeatures := flag.Bool("extract_features", false, "Run the feature extraction stage")
	flag.Parse()

	// Start timing
	startTime := time.Now()

	// Defer runtime logging to ensure it always runs
	defer func() {
		duration := time.Since(startTime)
		utils.Info.Printf("Total runtime: %v", duration)
	}()

	// Initialize logger and config
	utils.InitLogger()

	// Load configuration
	config := config.LoadConfig()

	if err := config.Validate(); err != nil {
		utils.Error.Fatal("Invalid configuration:", err)
	}

	// Log config values
	// Connect to postgres
	pool, err := db.ConnectPool(&config)
	if err != nil {
		utils.Error.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	utils.Info.Println("Database connection established")

	if *createDB {
		if err := db.CreateCity2TABULADB(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to create City2TABULA database: %v", err)
		}
	}

	if *resetDB {
		utils.Info.Println("Resetting City2TABULA database...")
		if err := db.Reset3DCityToTabulaDB(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to reset City2TABULA database: %v", err)
		}
		utils.Info.Println("City2TABULA reset completed successfully")
	}

	if *resetCity2Tabula {
		db.DropSchemaIfExists(pool, config.DB.Schemas.City2Tabula)
		db.DropSchemaIfExists(pool, config.DB.Schemas.Tabula)
		if err := db.CreateCity2TABULADB(&config, pool); err != nil {
			utils.Error.Fatalf("Failed to recreate City2TABULA database: %v", err)
		}
		utils.Info.Println("City2TABULA database recreated successfully")
	}
	if *extractFeatures {
		utils.Info.Println("Running feature extraction stage...")

		lod2BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod2)
		if err != nil {
			utils.Error.Fatalf("Failed to get LOD2 building IDs: %v", err)
		}
		utils.Info.Printf("Found %d buildings for lod2 in CityDB", len(lod2BuildingIDs))

		lod3BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, config.DB.Schemas.Lod3)
		if err != nil {
			utils.Error.Fatalf("Failed to get LOD3 building IDs: %v", err)
		}
		utils.Info.Printf("Found %d buildings for lod3 in CityDB", len(lod3BuildingIDs))

		batchesLOD2 := utils.CreateBatches(lod2BuildingIDs, config.Batch.Size)
		batchesLOD3 := utils.CreateBatches(lod3BuildingIDs, config.Batch.Size)

		if batchesLOD2 != nil {
			utils.Debug.Printf("Created %d batches for LOD2: %v", len(batchesLOD2), batchesLOD2[0])
		}
		if batchesLOD3 != nil {
			utils.Debug.Printf("Created %d batches for LOD3: %v", len(batchesLOD3), batchesLOD3[0])
		}

		pipelineQueue, err := process.BuildFeatureExtractionQueue(&config, batchesLOD2, batchesLOD3)
		if err != nil {
			utils.Error.Fatalf("Failed to build feature extraction queue: %v", err)
		}

		utils.PrintPipelineQueueInfo(pipelineQueue.Len(), len(pipelineQueue.Peek().Jobs))

		// make a channel for pipelines
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

		for i := 1; i <= numWorkers; i++ {
			wg.Add(1)
			worker := process.NewWorker(i)
			go worker.Start(pipelineChan, pool, &wg, &config)
		}

		wg.Wait()

	}
}
