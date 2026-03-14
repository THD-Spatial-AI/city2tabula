package process

import (
	"fmt"

	"github.com/THD-Spatial/City2TABULA/internal/config"
	"github.com/THD-Spatial/City2TABULA/internal/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RunFeatureExtraction(config *config.Config, pool *pgxpool.Pool) error {
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
	if len(lod2BldIDs)+len(lod3BldIDs) == 0 {
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
	pipQueue, err := BuildFeatureExtractionQueue(config, batchesLOD2, batchesLOD3)
	if err != nil {
		return fmt.Errorf("failed to build feature extraction queue: %w", err)
	}

	if pipQueue.Len() > 0 {
		utils.PrintPipelineQueueInfo(pipQueue.Len(), len(pipQueue.Peek().Jobs), config.Batch)
	} else {
		utils.Warn.Printf("Pipeline queue is empty - this shouldn't happen if buildings were found.")
	}

	return RunPipelineQueue(pipQueue, pool, config)
}
