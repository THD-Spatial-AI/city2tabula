package onrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// Building is one LOD2 or LOD3 building's thematic (non-geometric) 3D attributes,
// joined to its PyLovo OSM match. No geometry here on purpose — a
// calculation consumer doesn't need it; see BuildingGeometryByObjectIDs
// for that, fetched separately only when something (e.g. a frontend)
// actually wants to render it.
type Building struct {
	ObjectID  string `json:"object_id"`
	DatasetID string `json:"dataset_id"`
	OSMID     string `json:"osm_id"`
	// OSMIDs is every OSM way linked to this building, OSMID first and the
	// further ways by overlap, highest first. Empty for a bbox query.
	OSMIDs    []string `json:"osm_ids"`
	MatchType int16    `json:"match_type"`
	// FunctionCode is the source Building's CityGML function code, for example
	// the German ALKIS code 31001_1000. Null where the dataset carries none.
	FunctionCode *string `json:"function_code"`
	// UseType is FunctionCode mapped to the neutral use vocabulary, and
	// Residential whether that code's building contains dwellings. Without a
	// mapped code they come from the building's units in an attribute dataset,
	// such as BAG in the Netherlands. Both are null where neither gives a use.
	UseType     *string `json:"use_type"`
	Residential *bool   `json:"residential"`
	// useDatasetIDs are the attribute datasets UseType came from, for credits.
	useDatasetIDs     []string
	MinHeight         *float64  `json:"min_height,omitempty"`
	MaxHeight         *float64  `json:"max_height,omitempty"`
	RoomHeight        *float64  `json:"room_height,omitempty"`
	NumberOfStoreys   *int32    `json:"number_of_storeys,omitempty"`
	FootprintAreaSqm  *float64  `json:"footprint_area,omitempty"`
	RoofAreaSqm       *float64  `json:"area_total_roof,omitempty"`
	WallAreaSqm       *float64  `json:"area_total_wall,omitempty"`
	PartyWallAreaSqm  *float64  `json:"area_party_wall,omitempty"`
	FloorAreaSqm      *float64  `json:"area_total_floor,omitempty"`
	TabulaVariantCode *string   `json:"tabula_variant_code,omitempty"`
	Surfaces          []Surface `json:"surfaces,omitempty"`

	// Attached neighbours share a wall with this building; the ids are their
	// object_ids, for fetching their geometry.
	HasAttachedNeighbour   *bool    `json:"has_attached_neighbour,omitempty"`
	AttachedNeighbourClass *int32   `json:"attached_neighbour_class,omitempty"`
	AttachedNeighbourIDs   []string `json:"attached_neighbour_id,omitempty"`
}

// Surface is one envelope surface (wall, roof, or ground) belonging to a
// Building — the per-element area/azimuth/tilt that Building's own
// aggregate totals (RoofAreaSqm etc.) don't carry. A wall shared with an attached
// neighbour is served as its own piece with IsPartyWall set; it is not part of
// the building's area_total_wall. Type is the raw CityGML
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
	// Azimuth is -1 (undefined) for near-horizontal surfaces, deliberately
	// outside the 0-360 range so it cannot be read as a bearing. The CityGML
	// Energy ADE 1.0 Feature Catalogue uses 0 for the same case; convert.
	Azimuth *float64 `json:"azimuth,omitempty"`
	// Tilt: 0=vertical wall, 90=flat roof — the opposite of the common
	// building-energy convention (0=horizontal roof, 90=vertical wall);
	// invert before mapping to a schema that uses that convention.
	Tilt *float64 `json:"tilt,omitempty"`
	// AreaBelowPrecision marks a surface whose area rounds to 0.00 at the
	// 2-decimal precision City2TABULA records. The surface is real and its
	// geometry, tilt and azimuth are sound, so it still renders, but Area is not
	// a usable thermal area: exclude these from an energy calculation rather
	// than treating the zero as real. Slivers from wall and roof intersections
	// in the source model land here.
	AreaBelowPrecision *bool `json:"area_below_precision,omitempty"`
	IsValid            *bool `json:"is_valid,omitempty"`
	IsPlanar           *bool `json:"is_planar,omitempty"`
	// HorizontalExtent and SlopeExtent are the extents of the face along the
	// horizontal and along the fall line in its own plane, absent for faces
	// within about 10 degrees of horizontal; Height is its vertical extent. All in metres.
	HorizontalExtent *float64 `json:"horizontal_extent,omitempty"`
	SlopeExtent      *float64 `json:"slope_extent,omitempty"`
	Height           *float64 `json:"height,omitempty"`
	// IsPartyWall marks a piece shared with the attached neighbour NeighbourObjectID.
	IsPartyWall       *bool   `json:"is_party_wall,omitempty"`
	NeighbourObjectID *string `json:"neighbour_object_id,omitempty"`
}

// BuildingsByOSMIDs returns 3D attributes for every building in cfg's
// country linked to one of osmIDs, each building once however many of its OSM
// ways were asked for, so a caller summing the result counts no envelope twice.
// OSMID is the building's best way and OSMIDs every way linked to it. An osm_id
// with no link above the threshold matches nothing, the same outcome as a
// country/region with no coverage at all.
func BuildingsByOSMIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, osmIDs []string) ([]Building, error) {
	if len(osmIDs) == 0 {
		return nil, nil
	}
	sourceAttrs, err := sourceAttributeColumns(ctx, pool, cfg)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`
		WITH hit AS (
			SELECT DISTINCT object_id FROM %[2]s.building_link
			WHERE country_code = $1 AND osm_id = ANY($2) AND match_type IN (1, 4)
		),
		ways AS (
			SELECT object_id,
			       max(osm_id) FILTER (WHERE match_type = 1) AS primary_osm_id,
			       array_agg(osm_id ORDER BY match_type, match_confidence DESC, osm_id) AS osm_ids
			FROM %[2]s.building_link
			WHERE country_code = $1 AND match_type IN (1, 4)
			  AND object_id IN (SELECT object_id FROM hit)
			GROUP BY object_id
		)
		SELECT
			b.object_id, b.dataset_id, w.primary_osm_id, 1::smallint,
			b.min_height, b.max_height, b.room_height, b.number_of_storeys,
			b.footprint_area, b.area_total_roof, b.area_total_wall, b.area_party_wall, b.area_total_floor,
			b.tabula_variant_code, %[1]s, w.osm_ids, %[4]s
		FROM ways w
		JOIN %[3]s b ON b.object_id = w.object_id AND b.country_code = $1`,
		neighbourColumns, cfg.DB.Schemas.City2Tabula, allLODs(cfg, "building", buildingColumns), sourceAttrs,
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, osmIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query buildings for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	buildings, err := scanBuildingRows(rows, cfg.Country)
	if err != nil {
		return nil, err
	}
	if err := attachSurfaces(ctx, pool, cfg, buildings); err != nil {
		return nil, err
	}
	return buildings, nil
}

// BuildingsByObjectIDs returns 3D attributes for the buildings in cfg's country
// with the given object_ids, linked to PyLovo or not, each once. A linked
// building carries its ways as BuildingsByOSMIDs returns them, with match_type 1;
// a building without an OSM match has empty ways and match_type 2; one with no
// building_link row yet has empty ways and match_type 0. Unknown ids match nothing.
func BuildingsByObjectIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, objectIDs []string) ([]Building, error) {
	if len(objectIDs) == 0 {
		return nil, nil
	}
	sourceAttrs, err := sourceAttributeColumns(ctx, pool, cfg)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`
		WITH ways AS (
			SELECT object_id,
			       max(osm_id) FILTER (WHERE match_type = 1) AS primary_osm_id,
			       array_agg(osm_id ORDER BY match_type, match_confidence DESC, osm_id)
			           FILTER (WHERE match_type IN (1, 4)) AS osm_ids,
			       min(match_type) FILTER (WHERE match_type IN (1, 2)) AS match_type
			FROM %[2]s.building_link
			WHERE country_code = $1 AND object_id = ANY($2)
			GROUP BY object_id
		)
		SELECT
			b.object_id, b.dataset_id, COALESCE(w.primary_osm_id, ''), COALESCE(w.match_type, 0)::smallint,
			b.min_height, b.max_height, b.room_height, b.number_of_storeys,
			b.footprint_area, b.area_total_roof, b.area_total_wall, b.area_party_wall, b.area_total_floor,
			b.tabula_variant_code, %[1]s, COALESCE(w.osm_ids, ARRAY[]::text[]), %[4]s
		FROM %[3]s b
		LEFT JOIN ways w ON w.object_id = b.object_id
		WHERE b.country_code = $1 AND b.object_id = ANY($2)`,
		neighbourColumns, cfg.DB.Schemas.City2Tabula, allLODs(cfg, "building", buildingColumns), sourceAttrs,
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, objectIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query buildings by object_id for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	buildings, err := scanBuildingRows(rows, cfg.Country)
	if err != nil {
		return nil, err
	}
	if err := attachSurfaces(ctx, pool, cfg, buildings); err != nil {
		return nil, err
	}
	return buildings, nil
}

// BuildingsByBBox returns 3D attributes for every building in cfg's
// country whose footprint intersects bbox, independent of whether a PyLovo
// building_link row exists for it yet. osm_id/match_type are left at their
// zero value and osm_ids empty on every returned Building. Callers that need
// the PyLovo match should use BuildingsByOSMIDs instead.
func BuildingsByBBox(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, bbox Bbox) ([]Building, error) {
	sourceAttrs, err := sourceAttributeColumns(ctx, pool, cfg)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(`
		SELECT
			b.object_id, b.dataset_id, '', 0,
			b.min_height, b.max_height, b.room_height, b.number_of_storeys,
			b.footprint_area, b.area_total_roof, b.area_total_wall, b.area_party_wall, b.area_total_floor,
			b.tabula_variant_code, %s, ARRAY[]::text[], %s
		FROM %s b
		WHERE b.country_code = $1
		  AND b.building_footprint_geom IS NOT NULL
		  AND ST_Intersects(b.building_footprint_geom, ST_Transform(ST_MakeEnvelope($2,$3,$4,$5,4326), $6::int))`,
		neighbourColumns, sourceAttrs, allLODs(cfg, "building", buildingColumns),
	)

	rows, err := pool.Query(ctx, q, cfg.CountryCode, bbox.Xmin, bbox.Ymin, bbox.Xmax, bbox.Ymax, cfg.CityDB.SRID)
	if err != nil {
		return nil, fmt.Errorf("failed to query buildings in bbox for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	buildings, err := scanBuildingRows(rows, cfg.Country)
	if err != nil {
		return nil, err
	}
	if err := attachSurfaces(ctx, pool, cfg, buildings); err != nil {
		return nil, err
	}
	return buildings, nil
}

// buildingColumns and surfaceColumns are the columns the queries below read,
// named because a database built by an earlier release has columns added to
// its lod2_ tables that its lod3_ tables lack, so SELECT * cannot be unioned.
const (
	buildingColumns = "object_id, country_code, dataset_id, min_height, max_height, room_height, number_of_storeys, " +
		"footprint_area, area_total_roof, area_total_wall, area_party_wall, area_total_floor, tabula_variant_code, " +
		"has_attached_neighbour, attached_neighbour_class, attached_neighbour_id, building_footprint_geom"
	// neighbourColumns casts the ids because a database built by an earlier
	// release stores attached_neighbour_id as INTEGER[].
	neighbourColumns = "b.has_attached_neighbour, b.attached_neighbour_class, b.attached_neighbour_id::TEXT[]"
	surfaceColumns   = "id, building_object_id, surface_type, surface_area, azimuth, tilt, is_valid, is_planar, " +
		"horizontal_extent, slope_extent, height, is_party_wall, neighbour_object_id, geom"
)

// allLODs reads columns of one City2TABULA table across the LOD2 and LOD3
// schemas. A database holding both levels of one area returns those buildings
// twice.
func allLODs(cfg *config.Config, table, columns string) string {
	return fmt.Sprintf("(SELECT %[5]s FROM %[1]s.%[2]s_%[4]s UNION ALL SELECT %[5]s FROM %[1]s.%[3]s_%[4]s)",
		cfg.DB.Schemas.City2Tabula, cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3, table, columns)
}

// sourceAttributeColumns returns the two SQL expressions, over a building row
// aliased b, that read a building's attributes from outside City2TABULA's own
// tables: its function code and its building_use units.
func sourceAttributeColumns(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) (string, error) {
	functionCode, err := functionCodeColumn(ctx, pool, cfg)
	if err != nil {
		return "", err
	}
	units, err := buildingUseColumn(ctx, pool, cfg)
	if err != nil {
		return "", err
	}
	return functionCode + ", " + units, nil
}

// buildingUseColumn returns the SQL expression that reads a building's units
// from building_use as a JSON array of {dataset_id, code, area}, area summed per
// dataset and code, or NULL where it has none. A database without the table,
// which the import creates only where attribute data exists, gives NULL.
func buildingUseColumn(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) (string, error) {
	var present bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`,
		pgx.Identifier{cfg.DB.Schemas.City2Tabula, "building_use"}.Sanitize(),
	).Scan(&present); err != nil {
		return "", fmt.Errorf("failed to check for the building_use table: %w", err)
	}
	if !present {
		return "NULL::jsonb", nil
	}
	return fmt.Sprintf(`(SELECT jsonb_agg(jsonb_build_object('dataset_id', dataset_id, 'code', use_code, 'area', area))
			FROM (SELECT dataset_id, use_code, sum(floor_area) AS area FROM %s u
			      WHERE u.object_id = b.object_id GROUP BY dataset_id, use_code) s)`,
		pgx.Identifier{cfg.DB.Schemas.City2Tabula, "building_use"}.Sanitize()), nil
}

// functionCodeColumn returns the SQL expression, over a building row aliased b,
// that reads the source Building's CityGML function code from the LOD2 and LOD3
// source tables. An LOD whose source tables lack the columns read, such as a
// database restored with the City2TABULA schemas alone, contributes NULL instead
// of failing the query.
func functionCodeColumn(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) (string, error) {
	var parts []string
	for _, lod := range []string{cfg.DB.Schemas.Lod2, cfg.DB.Schemas.Lod3} {
		var present bool
		if err := pool.QueryRow(ctx, `
			SELECT count(*) = 4 FROM information_schema.columns
			WHERE table_schema = $1
			  AND (table_name, column_name) IN (('property', 'val_string'), ('property', 'parent_id'),
			                                    ('feature', 'objectid'), ('feature', 'objectclass_id'))`, lod,
		).Scan(&present); err != nil {
			return "", fmt.Errorf("failed to check the source tables of %s for function codes: %w", lod, err)
		}
		if !present {
			continue
		}
		schema := pgx.Identifier{lod}.Sanitize()
		parts = append(parts, fmt.Sprintf(`(SELECT p.val_string FROM %[1]s.feature f
			JOIN %[1]s.property p ON p.feature_id = f.id AND p.name = 'function' AND p.parent_id IS NULL
			WHERE f.objectid = b.object_id AND f.objectclass_id = 901
			ORDER BY p.id LIMIT 1)`, schema))
	}
	if len(parts) == 0 {
		return "NULL::text", nil
	}
	return "COALESCE(" + strings.Join(parts, ", ") + ")", nil
}

func scanBuildingRows(rows pgx.Rows, country string) ([]Building, error) {
	var buildings []Building
	for rows.Next() {
		var b Building
		var units []byte
		if err := rows.Scan(
			&b.ObjectID, &b.DatasetID, &b.OSMID, &b.MatchType,
			&b.MinHeight, &b.MaxHeight, &b.RoomHeight, &b.NumberOfStoreys,
			&b.FootprintAreaSqm, &b.RoofAreaSqm, &b.WallAreaSqm, &b.PartyWallAreaSqm, &b.FloorAreaSqm,
			&b.TabulaVariantCode,
			&b.HasAttachedNeighbour, &b.AttachedNeighbourClass, &b.AttachedNeighbourIDs,
			&b.OSMIDs, &b.FunctionCode, &units,
		); err != nil {
			return nil, fmt.Errorf("failed to scan building row: %w", err)
		}
		if b.OSMIDs == nil {
			b.OSMIDs = []string{}
		}
		b.UseType, b.Residential = useTypeFor(country, b.FunctionCode)
		if b.UseType == nil {
			use, residential, datasets, err := useTypeFromUnits(country, units)
			if err != nil {
				return nil, fmt.Errorf("building %s: %w", b.ObjectID, err)
			}
			b.UseType, b.Residential, b.useDatasetIDs = use, residential, datasets
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
	// area_below_precision is derived with the surface builder's own definition, since a
	// database built by an earlier release lacks the column on lod3_surface.
	q := fmt.Sprintf(`
		SELECT building_object_id, id::text, surface_type,
		       surface_area, azimuth, tilt,
		       (surface_area IS NOT NULL AND surface_area <= 0),
		       is_valid, is_planar, horizontal_extent, slope_extent, height, is_party_wall, neighbour_object_id
		FROM %s s
		WHERE building_object_id = ANY($1)`,
		allLODs(cfg, "surface", surfaceColumns),
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
			&s.AreaSqm, &s.Azimuth, &s.Tilt, &s.AreaBelowPrecision, &s.IsValid, &s.IsPlanar,
			&s.HorizontalExtent, &s.SlopeExtent, &s.Height, &s.IsPartyWall, &s.NeighbourObjectID,
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
	DatasetID        string          `json:"dataset_id"`
	FootprintGeoJSON json.RawMessage `json:"footprint_geojson,omitempty"`
	// Surfaces is populated only when the caller asks for it, since a single
	// building can carry a few hundred faces and most callers want the
	// footprint alone.
	Surfaces []SurfaceGeometry `json:"surfaces,omitempty"`
}

// SurfaceGeometry is one envelope surface's polygon, keyed by the same ID
// Surface carries, so a caller holding surfaces from a buildings query joins
// geometry onto them without a second identifier. IsPartyWall marks a piece shared
// with an attached neighbour, which a viewer can highlight.
//
// GeoJSON keeps its Z coordinates and the geometry's native CRS, the same as
// the footprint: no reprojection happens anywhere in the pipeline.
type SurfaceGeometry struct {
	ID                string          `json:"id"`
	Type              string          `json:"type"`
	IsPartyWall       *bool           `json:"is_party_wall,omitempty"`
	NeighbourObjectID *string         `json:"neighbour_object_id,omitempty"`
	GeoJSON           json.RawMessage `json:"geojson,omitempty"`
}

// geoJSON renders expr as GeoJSON in its storage CRS, which names that CRS in a
// crs member, or reprojected to EPSG:4326, which RFC 7946 assumes and so has none.
func geoJSON(expr string, wgs84 bool) string {
	if wgs84 {
		return "ST_AsGeoJSON(ST_Transform(" + expr + ", 4326))"
	}
	return "ST_AsGeoJSON(" + expr + ")"
}

// BuildingGeometryByObjectIDs returns footprint geometry for the given
// building object IDs in cfg's country. With includeSurfaces, each building
// also carries its individual envelope surface polygons. With wgs84, every
// geometry is reprojected to EPSG:4326; otherwise it stays in the storage CRS.
func BuildingGeometryByObjectIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, objectIDs []string, includeSurfaces, wgs84 bool) ([]BuildingGeometry, error) {
	if len(objectIDs) == 0 {
		return nil, nil
	}

	q := fmt.Sprintf(`
		SELECT object_id, dataset_id, COALESCE(%s, '')
		FROM %s b
		WHERE country_code = $1 AND object_id = ANY($2)`,
		geoJSON("ST_Force2D(building_footprint_geom)", wgs84), allLODs(cfg, "building", buildingColumns),
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
		if err := rows.Scan(&g.ObjectID, &g.DatasetID, &footprintGeoJSON); err != nil {
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

	if includeSurfaces {
		if err := attachSurfaceGeometry(ctx, pool, cfg, geometries, wgs84); err != nil {
			return nil, err
		}
	}
	return geometries, nil
}

// attachSurfaceGeometry fetches every surface polygon for geometries' object IDs
// in one batched query and sets each BuildingGeometry's Surfaces field in place,
// avoiding one query per building.
func attachSurfaceGeometry(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, geometries []BuildingGeometry, wgs84 bool) error {
	if len(geometries) == 0 {
		return nil
	}

	objectIDs := make([]string, len(geometries))
	byObjectID := make(map[string]*BuildingGeometry, len(geometries))
	for i := range geometries {
		objectIDs[i] = geometries[i].ObjectID
		byObjectID[geometries[i].ObjectID] = &geometries[i]
	}

	// No ST_Force2D here, unlike the footprint: the Z coordinate is the point of
	// asking for surfaces. id (row UUID), not surface_object_id, which one source
	// surface feature shares across all of its faces.
	q := fmt.Sprintf(`
		SELECT building_object_id, id::text, COALESCE(surface_type, ''),
		       is_party_wall, neighbour_object_id, COALESCE(%s, '')
		FROM %s s
		WHERE building_object_id = ANY($1)
		ORDER BY building_object_id, id`,
		geoJSON("geom", wgs84), allLODs(cfg, "surface", surfaceColumns),
	)

	rows, err := pool.Query(ctx, q, objectIDs)
	if err != nil {
		return fmt.Errorf("failed to query surface geometry for %s: %w", cfg.Country, err)
	}
	defer rows.Close()

	for rows.Next() {
		var buildingObjectID string
		var s SurfaceGeometry
		var geoJSON string
		if err := rows.Scan(&buildingObjectID, &s.ID, &s.Type, &s.IsPartyWall, &s.NeighbourObjectID, &geoJSON); err != nil {
			return fmt.Errorf("failed to scan surface geometry row: %w", err)
		}
		// Empty string (no geometry) stays nil, not an empty-but-non-nil
		// RawMessage, which encoding/json would reject as invalid JSON.
		if geoJSON != "" {
			s.GeoJSON = json.RawMessage(geoJSON)
		}
		if g, ok := byObjectID[buildingObjectID]; ok {
			g.Surfaces = append(g.Surfaces, s)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating surface geometry rows: %w", err)
	}
	return nil
}
