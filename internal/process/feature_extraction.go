package process

import (
	"context"
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

		ids, err = excludeProcessedBuildingIDs(pool, cfg, schema, ids)
		if err != nil {
			return fmt.Errorf("failed to filter already-processed LOD%d building IDs: %w", lod, err)
		}

		if len(ids) == 0 {
			utils.Warn.Printf("No LOD%d buildings to extract (none in CityDB, or all already processed). Skipping LOD%d feature extraction.", lod, lod)
			continue
		}
		utils.Info.Printf("Found %d buildings for LOD%d in CityDB", len(ids), lod)

		if limit := cfg.Batch.BuildingLimit; limit > 0 && limit < len(ids) {
			ids = ids[:limit]
			utils.Info.Printf("BUILDING_LIMIT=%d applied: processing %d of available buildings for LOD%d", limit, limit, lod)
		}

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

	if err := RunJobQueue(jobQueue, pool, cfg); err != nil {
		return err
	}

	// Correction triggers (sql/schema/main/03_create_correction_triggers.sql) are
	// installed disabled by -create-db, since they'd otherwise contend with this
	// same bulk run. Turn them on now that extraction has populated the table, so
	// manual corrections (e.g. in QGIS) work right away without a separate step.
	if len(lod2BldIDs) > 0 {
		if err := enableCorrectionTriggers(pool, cfg, cfg.DB.Schemas.Lod2); err != nil {
			return err
		}
	}
	if len(lod3BldIDs) > 0 {
		if err := enableCorrectionTriggers(pool, cfg, cfg.DB.Schemas.Lod3); err != nil {
			return err
		}
	}

	return nil
}

// excludeProcessedBuildingIDs drops any building_feature_id already present in
// {lodSchema}_building from ids, so a repeat -extract-features run doesn't re-run
// scripts 04-07 against buildings a user may have since hand-corrected — those
// scripts have no skip-already-processed filter of their own, and re-writing a
// corrected row would incorrectly mark it as freshly changed (see
// sql/schema/main/03_create_correction_triggers.sql's trg_touch_updated_at).
func excludeProcessedBuildingIDs(pool *pgxpool.Pool, cfg *config.Config, lodSchema string, ids []int64) ([]int64, error) {
	processed, err := getProcessedBuildingFeatureIDs(pool, cfg.DB.Schemas.City2Tabula, lodSchema)
	if err != nil {
		return nil, err
	}
	if len(processed) == 0 {
		return ids, nil
	}

	remaining := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !processed[id] {
			remaining = append(remaining, id)
		}
	}
	if skipped := len(ids) - len(remaining); skipped > 0 {
		utils.Info.Printf("Skipping %d already-processed buildings in %s (already have a %s_building row)", skipped, lodSchema, lodSchema)
	}
	return remaining, nil
}

// enableCorrectionTriggers turns on the five correction triggers for one LOD
// schema's _building table (see sql/schema/main/03_create_correction_triggers.sql
// for what each one recomputes; trg_touch_updated_at just stamps updated_at).
func enableCorrectionTriggers(pool *pgxpool.Pool, cfg *config.Config, lodSchema string) error {
	table := fmt.Sprintf("%s.%s_building", cfg.DB.Schemas.City2Tabula, lodSchema)
	triggers := []string{
		lodSchema + "_trg_footprint_geom_change",
		lodSchema + "_trg_variant_dims_change",
		lodSchema + "_trg_room_height_change",
		lodSchema + "_trg_storeys_change",
		lodSchema + "_trg_touch_updated_at",
	}

	// ALTER TABLE ... ENABLE TRIGGER only accepts one trigger name per statement,
	// unlike a column list — so each trigger needs its own ALTER TABLE call. All five
	// are wrapped in one transaction so a concurrent UPDATE against this table either
	// sees none of them enabled or all of them — never a partial state where, e.g.,
	// the correction-cascade triggers are live but trg_touch_updated_at isn't yet.
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin trigger-enable transaction on %s: %w", table, err)
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	for _, trigger := range triggers {
		query := fmt.Sprintf(`ALTER TABLE %s ENABLE TRIGGER %s;`, table, trigger)
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to enable trigger %s on %s: %w", trigger, table, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit trigger-enable transaction on %s: %w", table, err)
	}
	utils.Info.Printf("Correction triggers enabled on %s", table)
	return nil
}

// RunPyLovoLinkBuild populates city2tabula.building_link by spatially joining 3D building
// footprints against pylovo.res and pylovo.oth. Must be run after RunFeatureExtraction.
// Only LOD2 buildings are processed — the link table is keyed on object_id, which is
// LOD-agnostic, so a single LOD pass is sufficient.
//
// Buildings are batched by spatial grid cell (default 1 km²) so each batch covers a
// compact geographic area. This keeps the PyLovo bounding-box pre-filter tight and
// avoids scanning the full PyLovo table for every batch.
func RunPyLovoLinkBuild(cfg *config.Config, pool *pgxpool.Pool) error {
	batches, err := getGridBatches(
		pool,
		cfg.DB.Schemas.City2Tabula,
		cfg.DB.Schemas.Lod2,
		cfg.City2Tabula.LinkGridSize,
		cfg.Batch.BuildingLimit,
	)
	if err != nil {
		return fmt.Errorf("failed to build spatial grid batches: %w", err)
	}

	if len(batches) == 0 {
		utils.Warn.Println("No LOD2 buildings with footprints found. Nothing to link.")
		return nil
	}

	total := 0
	for _, b := range batches {
		total += len(b)
	}
	utils.Info.Printf("Spatial grid batching: %d grid cells, %d buildings total (grid size: %dm)",
		len(batches), total, cfg.City2Tabula.LinkGridSize)

	jobQueue, err := PyLovoLinkJobQueue(cfg, batches)
	if err != nil {
		return fmt.Errorf("failed to build PyLovo link job queue: %w", err)
	}

	if jobQueue.Len() > 0 {
		utils.PrintJobQueueInfo(jobQueue.Len(), len(jobQueue.Peek().Tasks), cfg.Batch)
	}

	return RunJobQueue(jobQueue, pool, cfg)
}

// getGridBatches divides LOD2 buildings into spatial batches using a square grid.
// Each returned slice contains the building_feature_ids that fall within one grid cell.
// Buildings with no footprint geometry or no object_id are excluded.
// If buildingLimit > 0, at most that many buildings are included in total.
func getGridBatches(pool *pgxpool.Pool, c2tSchema, lodSchema string, gridSizeM, buildingLimit int) ([][]int64, error) {
	limitClause := ""
	if buildingLimit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", buildingLimit)
	}

	// ST_SquareGrid requires PostGIS >= 3.1.
	// Buildings are grouped by grid cell; cells with no buildings are excluded.
	q := fmt.Sprintf(`
		WITH all_buildings AS (
			SELECT building_feature_id, ST_Force2D(building_footprint_geom) AS geom
			FROM %s.%s_building
			WHERE building_footprint_geom IS NOT NULL
			  AND object_id IS NOT NULL
			%s
		),
		extent AS (
			SELECT ST_Envelope(ST_Collect(geom)) AS bbox
			FROM all_buildings
		),
		grid AS (
			SELECT (ST_SquareGrid($1::double precision, bbox)).geom AS cell
			FROM extent
			WHERE bbox IS NOT NULL
		)
		SELECT array_agg(b.building_feature_id ORDER BY b.building_feature_id)
		FROM all_buildings b
		JOIN grid g ON ST_Intersects(b.geom, g.cell)
		GROUP BY g.cell
		HAVING count(*) > 0
	`, c2tSchema, lodSchema, limitClause)

	rows, err := pool.Query(context.Background(), q, gridSizeM)
	if err != nil {
		return nil, fmt.Errorf("grid batch query failed: %w", err)
	}
	defer rows.Close()

	var batches [][]int64
	for rows.Next() {
		var ids []int64
		if err := rows.Scan(&ids); err != nil {
			return nil, fmt.Errorf("scanning grid batch row: %w", err)
		}
		if len(ids) > 0 {
			batches = append(batches, ids)
		}
	}
	return batches, rows.Err()
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
