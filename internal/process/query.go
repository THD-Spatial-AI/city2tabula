package process

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getProcessedBuildingFeatureIDs returns the building_feature_id values already present
// in {city2tabulaSchema}.{lodSchema}_building, so a repeat -extract-features run can skip
// them instead of letting scripts 04-07 re-write rows a user may have since hand-corrected.
func getProcessedBuildingFeatureIDs(dbConn *pgxpool.Pool, city2tabulaSchema, lodSchema string) (map[int64]bool, error) {
	query := fmt.Sprintf(`
        SELECT building_feature_id
        FROM %s.%s_building
        WHERE building_feature_id IS NOT NULL`, city2tabulaSchema, lodSchema)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query processed building IDs: %w", err)
	}
	defer rows.Close()

	processed := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		processed[id] = true
	}
	return processed, rows.Err()
}

// getBuildingIDsFromCityDB returns the feature ids of every CityGML Building
// (objectclass 901) in schemaName. A BuildingPart is never a building of its own:
// script 01 reaches parts through their Building, so the Building is the unit
// every output row is keyed by, whichever of its features own the solids.
func getBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error) {
	query := fmt.Sprintf(`
        SELECT id
        FROM %s.feature
        WHERE objectclass_id = 901
        ORDER BY id`, schemaName)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building IDs in %s: %w", schemaName, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan building ID in %s: %w", schemaName, err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
