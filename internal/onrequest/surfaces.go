package onrequest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// renderedSurfaceTypes are the surfaces a 3D view draws; ground surfaces lie
// under the building and are left out.
var renderedSurfaceTypes = []string{"RoofSurface", "WallSurface"}

// BuildingSurfaces is one building's roof and wall surfaces for rendering.
type BuildingSurfaces struct {
	ObjectID  string `json:"object_id"`
	DatasetID string `json:"-"`
	// OSMIDs are the OSM ways linked to the building, best first; empty when
	// it has none.
	OSMIDs   []string        `json:"osm_ids"`
	Surfaces []RenderSurface `json:"surfaces"`
}

// RenderSurface is one surface polygon: the same id as SurfaceGeometry, its
// CityGML class, and a GeoJSON Polygon in EPSG:4326 with Z in metres.
type RenderSurface struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Geometry json.RawMessage `json:"geometry"`
}

// SurfacePage is one page of SurfacesInBBox. NextCursor is empty on the last page.
type SurfacePage struct {
	Buildings  []BuildingSurfaces
	NextCursor string
}

// EncodeSurfaceCursor and DecodeSurfaceCursor convert between the last object_id
// of a page and the opaque cursor a caller passes back for the next one.
func EncodeSurfaceCursor(objectID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(objectID))
}

// DecodeSurfaceCursor returns the object_id a cursor stands for; "" starts at the
// first page.
func DecodeSurfaceCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || (cursor != "" && len(b) == 0) {
		return "", fmt.Errorf("cursor %q is not one this API returned", cursor)
	}
	return string(b), nil
}

// SurfacesInBBox returns one page of buildings in cfg's country whose footprint
// intersects bbox, in object_id order after the building afterObjectID names, each
// with its roof and wall surfaces. A page holds whole buildings whose surfaces
// total at most maxVertices vertices, and at least one building.
func SurfacesInBBox(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, bbox Bbox, afterObjectID string, maxVertices int) (SurfacePage, error) {
	buildings, more, err := surfacePageBuildings(ctx, pool, cfg, bbox, afterObjectID, maxVertices)
	if err != nil || len(buildings) == 0 {
		return SurfacePage{Buildings: buildings}, err
	}
	if err := attachRenderSurfaces(ctx, pool, cfg, buildings); err != nil {
		return SurfacePage{}, err
	}
	if err := attachOSMIDs(ctx, pool, cfg, buildings); err != nil {
		return SurfacePage{}, err
	}
	page := SurfacePage{Buildings: buildings}
	if more {
		page.NextCursor = EncodeSurfaceCursor(buildings[len(buildings)-1].ObjectID)
	}
	return page, nil
}

// surfacePageBuildings selects the page's buildings by a running total of their
// surface vertices, so no geometry is encoded for buildings left to later pages.
// more reports whether buildings remain after the page.
func surfacePageBuildings(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, bbox Bbox, after string, maxVertices int) ([]BuildingSurfaces, bool, error) {
	q := fmt.Sprintf(`
		WITH b AS (
			SELECT object_id, dataset_id FROM %[1]s b
			WHERE country_code = $1 AND object_id > $2
			  AND building_footprint_geom IS NOT NULL
			  AND ST_Intersects(building_footprint_geom, ST_Transform(ST_MakeEnvelope($3,$4,$5,$6,4326), $7::int))
		),
		v AS (
			SELECT b.object_id, b.dataset_id, COALESCE(sum(ST_NPoints(s.geom)), 0) AS vertices
			FROM b LEFT JOIN %[2]s s ON s.building_object_id = b.object_id AND s.surface_type = ANY($8)
			GROUP BY b.object_id, b.dataset_id
		),
		c AS (
			SELECT object_id, dataset_id,
			       sum(vertices) OVER (ORDER BY object_id) AS running,
			       row_number() OVER (ORDER BY object_id) AS n,
			       count(*) OVER () AS total
			FROM v
		)
		SELECT object_id, dataset_id, total FROM c
		WHERE running <= $9 OR n = 1
		ORDER BY object_id`,
		allLODs(cfg, "building", buildingColumns), allLODs(cfg, "surface", surfaceColumns),
	)
	rows, err := pool.Query(ctx, q, cfg.CountryCode, after, bbox.Xmin, bbox.Ymin, bbox.Xmax, bbox.Ymax,
		cfg.CityDB.SRID, renderedSurfaceTypes, maxVertices)
	if err != nil {
		return nil, false, fmt.Errorf("failed to select surface page for %s: %w", cfg.Country, err)
	}
	defer rows.Close()

	var buildings []BuildingSurfaces
	var total int64
	for rows.Next() {
		var b BuildingSurfaces
		if err := rows.Scan(&b.ObjectID, &b.DatasetID, &total); err != nil {
			return nil, false, fmt.Errorf("failed to scan surface page row: %w", err)
		}
		b.OSMIDs, b.Surfaces = []string{}, []RenderSurface{}
		buildings = append(buildings, b)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("error iterating surface page rows: %w", err)
	}
	return buildings, total > int64(len(buildings)), nil
}

// attachRenderSurfaces fills each building's roof and wall surfaces in one query.
func attachRenderSurfaces(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, buildings []BuildingSurfaces) error {
	byID, ids := indexSurfaceBuildings(buildings)
	q := fmt.Sprintf(`
		SELECT building_object_id, id::text, surface_type, ST_AsGeoJSON(ST_Transform(geom, 4326), 7)
		FROM %s s
		WHERE building_object_id = ANY($1) AND surface_type = ANY($2) AND geom IS NOT NULL
		ORDER BY building_object_id, id`,
		allLODs(cfg, "surface", surfaceColumns),
	)
	rows, err := pool.Query(ctx, q, ids, renderedSurfaceTypes)
	if err != nil {
		return fmt.Errorf("failed to query render surfaces for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	for rows.Next() {
		var objectID, geometry string
		var s RenderSurface
		if err := rows.Scan(&objectID, &s.ID, &s.Type, &geometry); err != nil {
			return fmt.Errorf("failed to scan render surface row: %w", err)
		}
		s.Geometry = json.RawMessage(geometry)
		byID[objectID].Surfaces = append(byID[objectID].Surfaces, s)
	}
	return rows.Err()
}

// attachOSMIDs fills each building's linked OSM ways, best first, as
// BuildingsByOSMIDs orders them.
func attachOSMIDs(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, buildings []BuildingSurfaces) error {
	byID, ids := indexSurfaceBuildings(buildings)
	q := fmt.Sprintf(`
		SELECT object_id, array_agg(osm_id ORDER BY match_type, match_confidence DESC, osm_id)
		FROM %s.building_link
		WHERE country_code = $1 AND object_id = ANY($2) AND match_type IN (1, 4)
		GROUP BY object_id`,
		cfg.DB.Schemas.City2Tabula,
	)
	rows, err := pool.Query(ctx, q, cfg.CountryCode, ids)
	if err != nil {
		return fmt.Errorf("failed to query OSM ways for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	for rows.Next() {
		var objectID string
		var osmIDs []string
		if err := rows.Scan(&objectID, &osmIDs); err != nil {
			return fmt.Errorf("failed to scan OSM way row: %w", err)
		}
		byID[objectID].OSMIDs = osmIDs
	}
	return rows.Err()
}

func indexSurfaceBuildings(buildings []BuildingSurfaces) (map[string]*BuildingSurfaces, []string) {
	byID := make(map[string]*BuildingSurfaces, len(buildings))
	ids := make([]string, len(buildings))
	for i := range buildings {
		byID[buildings[i].ObjectID] = &buildings[i]
		ids[i] = buildings[i].ObjectID
	}
	return byID, ids
}

// SurfaceDatasetIDs returns the distinct datasets behind buildings, for credits.
func SurfaceDatasetIDs(buildings []BuildingSurfaces) []string {
	ids := make([]string, 0, len(buildings))
	for _, b := range buildings {
		ids = append(ids, b.DatasetID)
	}
	return distinct(ids)
}
