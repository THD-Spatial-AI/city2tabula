package onrequest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// Building is one LOD2 building's thematic (non-geometric) 3D attributes,
// joined to its PyLovo OSM match. No geometry here on purpose — a
// calculation consumer doesn't need it; see BuildingGeometryByObjectIDs
// for that, fetched separately only when something (e.g. a frontend)
// actually wants to render it.
type Building struct {
	ObjectID          string    `json:"object_id"`
	OSMID             string    `json:"osm_id"`
	MatchType         int16     `json:"match_type"`
	MinHeight         *float64  `json:"min_height,omitempty"`
	MaxHeight         *float64  `json:"max_height,omitempty"`
	RoomHeight        *float64  `json:"room_height,omitempty"`
	NumberOfStoreys   *int32    `json:"number_of_storeys,omitempty"`
	FootprintAreaSqm  *float64  `json:"footprint_area,omitempty"`
	RoofAreaSqm       *float64  `json:"area_total_roof,omitempty"`
	WallAreaSqm       *float64  `json:"area_total_wall,omitempty"`
	FloorAreaSqm      *float64  `json:"area_total_floor,omitempty"`
	TabulaVariantCode *string   `json:"tabula_variant_code,omitempty"`
	Surfaces          []Surface `json:"surfaces,omitempty"`
}

// Surface is one envelope surface (wall, roof, or ground) belonging to a
// Building — the per-element area/azimuth/tilt that Building's own
// aggregate totals (RoofAreaSqm etc.) don't carry. Party walls are already
// excluded upstream (sql/scripts/main/08_build_surface.sql), so every
// surface here is exposed building fabric. Type is the raw CityGML
// classname (WallSurface, RoofSurface, GroundSurface) — callers map this
// onto whatever vocabulary their own schema expects. IsValid/IsPlanar are
// not filtered here; callers should check them before trusting
// Area/Azimuth/Tilt, since a degenerate source surface can still produce a
// row.
type Surface struct {
	// Row identifier, unique per face. Not the CityGML surface id, which is
	// shared by every face of a multi-face surface feature.
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	AreaSqm *float64 `json:"area,omitempty"`
	// Azimuth is -1 (undefined) for near-horizontal surfaces.
	Azimuth *float64 `json:"azimuth,omitempty"`
	// Tilt: 0=vertical wall, 90=flat roof — the opposite of the common
	// building-energy convention (0=horizontal roof, 90=vertical wall);
	// invert before mapping to a schema that uses that convention.
	Tilt     *float64 `json:"tilt,omitempty"`
	IsValid  *bool    `json:"is_valid,omitempty"`
	IsPlanar *bool    `json:"is_planar,omitempty"`
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
			b.tabula_variant_code
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
	buildings, err := scanBuildingRows(rows)
	if err != nil {
		return nil, err
	}
	if err := attachSurfaces(ctx, pool, cfg, buildings); err != nil {
		return nil, err
	}
	return buildings, nil
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
			b.tabula_variant_code
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
	buildings, err := scanBuildingRows(rows)
	if err != nil {
		return nil, err
	}
	if err := attachSurfaces(ctx, pool, cfg, buildings); err != nil {
		return nil, err
	}
	return buildings, nil
}

func scanBuildingRows(rows pgx.Rows) ([]Building, error) {
	var buildings []Building
	for rows.Next() {
		var b Building
		if err := rows.Scan(
			&b.ObjectID, &b.OSMID, &b.MatchType,
			&b.MinHeight, &b.MaxHeight, &b.RoomHeight, &b.NumberOfStoreys,
			&b.FootprintAreaSqm, &b.RoofAreaSqm, &b.WallAreaSqm, &b.FloorAreaSqm,
			&b.TabulaVariantCode,
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

// attachSurfaces fetches every surface for buildings' object IDs in one
// batched query and sets each Building's Surfaces field in place, avoiding
// one query per building.
func attachSurfaces(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, buildings []Building) error {
	if len(buildings) == 0 {
		return nil
	}

	objectIDs := make([]string, len(buildings))
	byObjectID := make(map[string]*Building, len(buildings))
	for i := range buildings {
		objectIDs[i] = buildings[i].ObjectID
		byObjectID[buildings[i].ObjectID] = &buildings[i]
	}

	// id (row UUID), not surface_object_id: one source surface feature has many
	// faces and shares its surface_object_id across all of them.
	q := fmt.Sprintf(`
		SELECT building_object_id, id::text, surface_type,
		       surface_area, azimuth, tilt, is_valid, is_planar
		FROM %s.%s_surface
		WHERE building_object_id = ANY($1)`,
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2,
	)

	rows, err := pool.Query(ctx, q, objectIDs)
	if err != nil {
		return fmt.Errorf("failed to query surfaces for %s: %w", cfg.Country, err)
	}
	defer rows.Close()

	for rows.Next() {
		var buildingObjectID string
		var s Surface
		if err := rows.Scan(
			&buildingObjectID, &s.ID, &s.Type,
			&s.AreaSqm, &s.Azimuth, &s.Tilt, &s.IsValid, &s.IsPlanar,
		); err != nil {
			return fmt.Errorf("failed to scan surface row: %w", err)
		}
		if b, ok := byObjectID[buildingObjectID]; ok {
			b.Surfaces = append(b.Surfaces, s)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating surface rows: %w", err)
	}
	return nil
}

// BuildingGeometry is one building's footprint geometry, fetched separately
// from Building — nothing in the calculation path needs it, only a
// visualization consumer, so it's not bundled into every buildings query.
type BuildingGeometry struct {
	ObjectID         string          `json:"object_id"`
	FootprintGeoJSON json.RawMessage `json:"footprint_geojson,omitempty"`
}

// BuildingGeometryByObjectIDs returns footprint geometry for the given
// building object IDs in cfg's country.
func BuildingGeometryByObjectIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, objectIDs []string) ([]BuildingGeometry, error) {
	if len(objectIDs) == 0 {
		return nil, nil
	}

	q := fmt.Sprintf(`
		SELECT object_id, COALESCE(ST_AsGeoJSON(ST_Force2D(building_footprint_geom)), '')
		FROM %s.%s_building
		WHERE country_code = $1 AND object_id = ANY($2)`,
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2,
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, objectIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query building geometry for %s: %w", cfg.Country, err)
	}
	defer rows.Close()

	var geometries []BuildingGeometry
	for rows.Next() {
		var g BuildingGeometry
		var footprintGeoJSON string
		if err := rows.Scan(&g.ObjectID, &footprintGeoJSON); err != nil {
			return nil, fmt.Errorf("failed to scan building geometry row: %w", err)
		}
		// Empty string (no geometry) stays nil, not an empty-but-non-nil
		// RawMessage, which encoding/json would reject as invalid JSON.
		if footprintGeoJSON != "" {
			g.FootprintGeoJSON = json.RawMessage(footprintGeoJSON)
		}
		geometries = append(geometries, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating building geometry rows: %w", err)
	}
	return geometries, nil
}
