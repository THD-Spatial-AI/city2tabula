CREATE OR REPLACE FUNCTION {city2tabula_schema}.surface_area_corrected_geom(
    geom geometry,
    nx double precision,
    ny double precision,
    nz double precision
)
RETURNS double precision AS $$
DECLARE
    projected_geom geometry;
    result_area double precision;
BEGIN
    IF geom IS NULL THEN
        RETURN 0.0;
    END IF;

    -- If geometry collapses in XY, handle as vertical wall
    IF ST_NPoints(ST_ConvexHull(ST_Force2D(geom))) < 3 THEN
        projected_geom := ST_RotateY(
            ST_RotateX(geom, atan2(ny, nz)),
            -atan2(nx, sqrt(ny*ny + nz*nz))
        );
        RETURN ST_Area(ST_Force2D(projected_geom));
    END IF;

    -- Standard rotation flatten
    projected_geom := ST_RotateY(
        ST_RotateX(geom, atan2(ny, nz)),
        -atan2(nx, sqrt(ny*ny + nz*nz))
    );
    result_area := ST_Area(ST_Force2D(projected_geom));

    -- Final fallback: height × max XY distance
    IF result_area = 0.0 THEN
        RETURN (
            WITH pts AS (
                SELECT (dp.geom) AS pt
                FROM ST_DumpPoints(geom) dp
            ),
            xy_pts AS (
                SELECT ST_Force2D(pt) AS pt2d, ST_Z(pt) AS z FROM pts
            ),
            height AS (
                SELECT MAX(z) - MIN(z) AS h FROM xy_pts
            ),
            width AS (
                SELECT MAX(ST_Distance(p1.pt2d, p2.pt2d)) AS w
                FROM xy_pts p1, xy_pts p2
            )
            SELECT h.h * w.w FROM height h, width w
        );
    END IF;

    RETURN COALESCE(GREATEST(result_area, 0.0), 0.0);
EXCEPTION
    WHEN OTHERS THEN
        RETURN 0.0;
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT;

-- Extent of a face along two axes in its plane, fixed by its unit normal n:
--   u, horizontal along the face: (-ny, nx, 0) / h, with h = sqrt(nx^2 + ny^2)
--   v, fall line pointing up the slope: (-nz*nx / h, -nz*ny / h, h)
-- Each extent is max minus min of the vertices projected on the axis, so the sign
-- of n does not matter. Faces with |nz| > 0.985, where azimuth is -1, have no
-- horizontal axis in their plane and get NULL in both.
-- DROP because CREATE OR REPLACE cannot change the OUT columns of an existing function.
DROP FUNCTION IF EXISTS {city2tabula_schema}.surface_dimensions(
    geometry, double precision, double precision, double precision
);
CREATE FUNCTION {city2tabula_schema}.surface_dimensions(
    geom geometry,
    nx double precision,
    ny double precision,
    nz double precision,
    OUT horizontal_extent double precision,
    OUT slope_extent double precision
) AS $$
    WITH axes AS (
        SELECT -ny / h AS ux, nx / h AS uy,
               -nz * nx / h AS vx, -nz * ny / h AS vy, h AS vz
        -- NULLIF because PostgreSQL 18 computes the axes of a flat face (h = 0)
        -- before the WHERE below discards it, and raises division by zero.
        FROM (SELECT NULLIF(sqrt(nx * nx + ny * ny), 0) AS h) s
        WHERE ABS(nz) <= 0.985
    ),
    projected AS (
        SELECT ST_X(dp.geom) * ux + ST_Y(dp.geom) * uy AS u,
               ST_X(dp.geom) * vx + ST_Y(dp.geom) * vy + ST_Z(dp.geom) * vz AS v
        FROM axes, ST_DumpPoints(geom) dp
    )
    SELECT MAX(u) - MIN(u), MAX(v) - MIN(v) FROM projected;
$$ LANGUAGE sql IMMUTABLE STRICT;
