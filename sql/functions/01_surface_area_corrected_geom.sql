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

-- Long and short side of the minimum-area rectangle around a face, measured in
-- the face's own plane. Rotated first because ST_OrientedEnvelope is 2D only and
-- a plan projection shortens a pitched roof's slope side by cos(slope).
-- A degenerate face whose envelope collapses to a line or point has width 0.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.surface_dimensions(
    geom geometry,
    nx double precision,
    ny double precision,
    nz double precision,
    OUT length double precision,
    OUT width double precision
) AS $$
DECLARE
    env geometry;
    side_a double precision;
    side_b double precision;
BEGIN
    env := ST_OrientedEnvelope(ST_Force2D(ST_RotateY(
        ST_RotateX(geom, atan2(ny, nz)),
        -atan2(nx, sqrt(ny*ny + nz*nz))
    )));

    IF GeometryType(env) <> 'POLYGON' THEN
        length := ST_Length(env);
        width := 0.0;
        RETURN;
    END IF;

    side_a := ST_Distance(ST_PointN(ST_ExteriorRing(env), 1), ST_PointN(ST_ExteriorRing(env), 2));
    side_b := ST_Distance(ST_PointN(ST_ExteriorRing(env), 2), ST_PointN(ST_ExteriorRing(env), 3));
    length := GREATEST(side_a, side_b);
    width := LEAST(side_a, side_b);
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT;
