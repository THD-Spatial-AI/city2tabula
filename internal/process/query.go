package process

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getBuildingObjectClassIDs(dbConn *pgxpool.Pool, schemaName string) ([]int, error) {
	query := fmt.Sprintf(`
        SELECT DISTINCT objectclass_id
        FROM %s.feature
        WHERE objectclass_id BETWEEN 900 AND 999
        ORDER BY objectclass_id`, schemaName)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building objectclass_ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func getBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error) {
	buildingClasses, err := getBuildingObjectClassIDs(dbConn, schemaName)
	if err != nil {
		return nil, err
	}

	if len(buildingClasses) == 0 {
		return []int64{}, nil
	}

	classList := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(buildingClasses)), ","), "[]")

	query := fmt.Sprintf(`
        SELECT id
        FROM %s.feature
        WHERE objectclass_id IN (%s)
        ORDER BY id`, schemaName, classList)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
