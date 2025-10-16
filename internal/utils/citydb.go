package utils

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func GetBuildingFeatureCount(dbConn *pgxpool.Pool, schemaName string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.feature WHERE objectclass_id IN (901, 905)`, schemaName)
	var count int
	err := dbConn.QueryRow(context.Background(), query).Scan(&count)
	return count, err
}

// GetBuildingIDsFromCityDB fetches building feature IDs from specified LOD schema
func GetBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error) {
	// Query to get building feature IDs (objectclass_id 901 for building feature and 905 for building part feature)
	query := fmt.Sprintf(`
		SELECT f.id
		FROM %s.feature f
		WHERE f.objectclass_id IN (901, 905)
		ORDER BY f.id`, schemaName)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building IDs from %s: %w", schemaName, err)
	}
	defer rows.Close()

	var buildingIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan building ID: %w", err)
		}
		buildingIDs = append(buildingIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return buildingIDs, nil
}
