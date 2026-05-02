package process

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// RunFeatureExtraction runs the SQL extraction pipeline for every LOD level
// listed in config.CityDB.LODLevels. Schemas that are not configured are never
// queried, so tests only need to seed the schemas they care about.
func RunFeatureExtraction(cfg *config.Config, pool *pgxpool.Pool) error {
	var lod2BldIDs, lod3BldIDs []int64

	for _, lod := range cfg.CityDB.LODLevels {
		schema, err := lodSchema(cfg, lod)
		if err != nil {
			return err
		}

		ids, err := getBuildingIDsFromCityDB(pool, schema)
		if err != nil {
			return fmt.Errorf("failed to get LOD%d building IDs: %w", lod, err)
		}

		if len(ids) == 0 {
			utils.Warn.Printf("No LOD%d buildings found in CityDB. Skipping LOD%d feature extraction.", lod, lod)
			continue
		}
		utils.Info.Printf("Found %d buildings for LOD%d in CityDB", len(ids), lod)

		switch lod {
		case 2:
			lod2BldIDs = ids
		case 3:
			lod3BldIDs = ids
		}
	}

	if len(lod2BldIDs)+len(lod3BldIDs) == 0 {
		utils.Warn.Println("No buildings found in any configured LOD schema. Nothing to extract.")
		return nil
	}

	batchesLOD2 := utils.CreateBatches(lod2BldIDs, cfg.Batch.Size)
	batchesLOD3 := utils.CreateBatches(lod3BldIDs, cfg.Batch.Size)

	jobQueue, err := BuildFeatureExtractionQueue(cfg, batchesLOD2, batchesLOD3)
	if err != nil {
		return fmt.Errorf("failed to build feature extraction queue: %w", err)
	}

	if jobQueue.Len() > 0 {
		utils.PrintJobQueueInfo(jobQueue.Len(), len(jobQueue.Peek().Tasks), cfg.Batch)
	}

	return RunJobQueue(jobQueue, pool, cfg)
}

// lodSchema returns the database schema name for the given LOD level.
func lodSchema(cfg *config.Config, lod int) (string, error) {
	switch lod {
	case 2:
		return cfg.DB.Schemas.Lod2, nil
	case 3:
		return cfg.DB.Schemas.Lod3, nil
	default:
		return "", fmt.Errorf("unsupported LOD level: %d", lod)
	}
}
