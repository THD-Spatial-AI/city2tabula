package process

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// getProcessedBuildingFeatureIDs returns the building_feature_id values already present
// in {city2tabulaSchema}.{lodSchema}_building, so a repeat -extract-features run can skip
// them instead of letting scripts 04-06 re-write rows a user may have since hand-corrected.
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

// buildingIDs returns the feature ids of the Buildings to extract from schema:
// all of them when area is nil, otherwise those getBuildingIDsInArea returns.
func buildingIDs(dbConn *pgxpool.Pool, cfg *config.Config, schema string, area *Area) ([]int64, error) {
	if area == nil {
		return getBuildingIDsFromCityDB(dbConn, schema)
	}
	return getBuildingIDsInArea(dbConn, schema, cfg.CityDB.SRID, *area)
}

// getBuildingIDsInArea returns the Buildings of schemaName whose envelope
// intersects area, the box citydb-tool's intersects filter tests, and every
// Building whose envelope lies within 0.1 m of theirs. 0.1 m is the gap
// sql/scripts/post/01_detect_neighbours.sql treats as attached; keep both equal.
func getBuildingIDsInArea(dbConn *pgxpool.Pool, schemaName, srid string, area Area) ([]int64, error) {
	query := fmt.Sprintf(`
        WITH inside AS (
            SELECT id, envelope
            FROM %[1]s.feature
            WHERE objectclass_id = 901
              AND ST_Intersects(envelope, ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, 4326), $5::int))
        )
        SELECT id FROM inside
        UNION
        SELECT f.id
        FROM %[1]s.feature f
        JOIN inside i ON ST_DWithin(f.envelope, i.envelope, 0.1)
        WHERE f.objectclass_id = 901
        ORDER BY id`, schemaName)
	return queryIDs(dbConn, schemaName, query, area.Xmin, area.Ymin, area.Xmax, area.Ymax, srid)
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

	return queryIDs(dbConn, schemaName, query)
}

// getPostScope returns the building_feature_ids of extracted and of every
// building in {lodSchema}_building whose footprint lies within 0.1 m of one of
// theirs, the gap sql/scripts/post/01_detect_neighbours.sql treats as attached.
func getPostScope(dbConn *pgxpool.Pool, city2tabulaSchema, lodSchema string, extracted []int64) ([]int64, error) {
	query := fmt.Sprintf(`
        SELECT DISTINCT c.building_feature_id
        FROM %[1]s.%[2]s_building a
        JOIN %[1]s.%[2]s_building c
          ON ST_DWithin(a.building_footprint_geom, c.building_footprint_geom, 0.1)
        WHERE a.building_feature_id = ANY($1)
        UNION
        SELECT unnest($1::bigint[])
        ORDER BY 1`, city2tabulaSchema, lodSchema)
	return queryIDs(dbConn, lodSchema+"_building", query, extracted)
}

// queryIDs runs a query returning one id column over schemaName's features.
func queryIDs(dbConn *pgxpool.Pool, schemaName, query string, args ...any) ([]int64, error) {
	rows, err := dbConn.Query(context.Background(), query, args...)
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
