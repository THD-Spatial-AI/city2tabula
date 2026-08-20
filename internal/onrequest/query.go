package onrequest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// Building is one LOD2 building's 3D attributes, joined to its PyLovo OSM match.
// Callers (the EnerPlanET backend's BuEM client) map these onto whatever shape
// BuEM's envelope block actually needs — this stays a raw projection of the
// underlying columns, not pre-shaped for one specific consumer.
type Building struct {
	ObjectID          string   `json:"object_id"`
	OSMID             string   `json:"osm_id"`
	MatchType         int16    `json:"match_type"`
	MinHeight         *float64 `json:"min_height,omitempty"`
	MaxHeight         *float64 `json:"max_height,omitempty"`
	RoomHeight        *float64 `json:"room_height,omitempty"`
	NumberOfStoreys   *int32   `json:"number_of_storeys,omitempty"`
	FootprintAreaSqm  *float64 `json:"footprint_area,omitempty"`
	RoofAreaSqm       *float64 `json:"area_total_roof,omitempty"`
	WallAreaSqm       *float64 `json:"area_total_wall,omitempty"`
	FloorAreaSqm      *float64 `json:"area_total_floor,omitempty"`
	TabulaVariantCode *string  `json:"tabula_variant_code,omitempty"`
	FootprintGeoJSON  string   `json:"footprint_geojson,omitempty"`
}

// BuildingsByOSMIDs returns 3D attributes for every LOD2 building in cfg's
// country whose building_link row matches one of osmIDs. Buildings with no
// match_type=1 link (no OSM counterpart found) are silently absent from the
// result — callers should treat a missing osm_id as "no 3D data for this
// building", the same outcome as a country/region with no coverage at all.
func BuildingsByOSMIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, osmIDs []string) ([]Building, error) {
	if len(osmIDs) == 0 {
		return nil, nil
	}

	q := fmt.Sprintf(`
		SELECT
			b.object_id, bl.osm_id, bl.match_type,
			b.min_height, b.max_height, b.room_height, b.number_of_storeys,
			b.footprint_area, b.area_total_roof, b.area_total_wall, b.area_total_floor,
			b.tabula_variant_code,
			COALESCE(ST_AsGeoJSON(ST_Force2D(b.building_footprint_geom)), '')
		FROM %[1]s.building_link bl
		JOIN %[1]s.%[2]s_building b ON b.object_id = bl.object_id AND b.country_code = bl.country_code
		WHERE bl.country_code = $1 AND bl.osm_id = ANY($2)`,
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2,
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, osmIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query buildings for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	return scanBuildingRows(rows)
}

// BuildingsByBBox returns 3D attributes for every LOD2 building in cfg's
// country whose footprint intersects bbox, independent of whether a PyLovo
// building_link row exists for it yet. osm_id/match_type are left at their
// zero value on every returned Building — callers that need the PyLovo
// match should use BuildingsByOSMIDs instead.
func BuildingsByBBox(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, bbox Bbox) ([]Building, error) {
	q := fmt.Sprintf(`
		SELECT
			b.object_id, '', 0,
			b.min_height, b.max_height, b.room_height, b.number_of_storeys,
			b.footprint_area, b.area_total_roof, b.area_total_wall, b.area_total_floor,
			b.tabula_variant_code,
			COALESCE(ST_AsGeoJSON(ST_Force2D(b.building_footprint_geom)), '')
		FROM %[1]s.%[2]s_building b
		WHERE b.country_code = $1
		  AND b.building_footprint_geom IS NOT NULL
		  AND ST_Intersects(b.building_footprint_geom, ST_Transform(ST_MakeEnvelope($2,$3,$4,$5,4326), $6::int))`,
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2,
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, bbox.Xmin, bbox.Ymin, bbox.Xmax, bbox.Ymax, cfg.CityDB.SRID)
	if err != nil {
		return nil, fmt.Errorf("failed to query buildings in bbox for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	return scanBuildingRows(rows)
}

func scanBuildingRows(rows pgx.Rows) ([]Building, error) {
	var buildings []Building
	for rows.Next() {
		var b Building
		if err := rows.Scan(
			&b.ObjectID, &b.OSMID, &b.MatchType,
			&b.MinHeight, &b.MaxHeight, &b.RoomHeight, &b.NumberOfStoreys,
			&b.FootprintAreaSqm, &b.RoofAreaSqm, &b.WallAreaSqm, &b.FloorAreaSqm,
			&b.TabulaVariantCode, &b.FootprintGeoJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan building row: %w", err)
		}
		buildings = append(buildings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating building rows: %w", err)
	}
	return buildings, nil
}
