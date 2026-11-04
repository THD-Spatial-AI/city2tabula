package importer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// catalogueSchema holds the file extent cache in the catalogue database, the
// database the server keeps for data on disk that belongs to no country yet.
const catalogueSchema = "public"

// footprintTolerance is the simplification tolerance of a footprint in degrees,
// about 100 m.
const footprintTolerance = 0.001

// PrepareCatalogue creates the file extent cache in the catalogue database
// behind conn if it is missing.
func PrepareCatalogue(ctx context.Context, conn *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		return createFileExtentTable(ctx, tx, catalogueSchema)
	})
}

// AddFootprints sets each entry's Footprint to the merged outline of the
// extents its model files declare, read through the file extent cache in the
// catalogue database behind conn. An entry none of whose files declares a
// readable extent in a recognised CRS gets none.
func AddFootprints(conn *pgxpool.Pool, list []DatasetRegion) error {
	for i := range list {
		files, err := modelFiles(list[i].folder)
		if err != nil {
			return err
		}
		extents, err := fileExtents(conn, catalogueSchema, list[i].Format, files)
		if err != nil {
			return err
		}
		if list[i].Footprint, err = footprint(conn, extents); err != nil {
			return fmt.Errorf("footprint of %s: %w", list[i].folder, err)
		}
	}
	return nil
}

// footprint returns the union of extents as a GeoJSON MultiPolygon in
// EPSG:4326, or nil when none is usable. Extents are merged per CRS before the
// transform, so the shared edges of adjacent tiles cancel exactly.
func footprint(conn *pgxpool.Pool, extents []declaredExtent) (json.RawMessage, error) {
	var minX, minY, maxX, maxY []float64
	var srids []int
	for _, e := range extents {
		if !e.ok || e.minX >= e.maxX || e.minY >= e.maxY {
			continue
		}
		epsg, err := resolveCRS(e.crs)
		if err != nil {
			continue // an unrecognised CRS cannot be placed; bbox selection skips it the same way
		}
		minX, minY, maxX, maxY = append(minX, e.minX), append(minY, e.minY), append(maxX, e.maxX), append(maxY, e.maxY)
		srids = append(srids, epsg)
	}
	if len(srids) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), extentTimeout)
	defer cancel()
	var geoJSON *string
	err := conn.QueryRow(ctx, `
		SELECT ST_AsGeoJSON(ST_Multi(ST_SimplifyPreserveTopology(ST_Union(g), $6)), 6)
		FROM (
			SELECT ST_Transform(ST_Union(ST_MakeEnvelope(x0, y0, x1, y1, srid)), 4326) AS g
			FROM unnest($1::float8[], $2::float8[], $3::float8[], $4::float8[], $5::int[]) AS t(x0, y0, x1, y1, srid)
			GROUP BY srid
		) s`, minX, minY, maxX, maxY, srids, footprintTolerance).Scan(&geoJSON)
	if err != nil || geoJSON == nil {
		return nil, err
	}
	return json.RawMessage(*geoJSON), nil
}
