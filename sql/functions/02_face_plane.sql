-- Rotates a face so its unit normal (nx, ny, nz) points along +Z, the same rotation
-- surface_area_corrected_geom and surface_dimensions use. In the rotated frame a
-- planar face lies at constant Z, so two faces with parallel normals can be
-- compared as 2D polygons and their plane offset read off as a Z difference.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.face_to_plane(
    geom geometry,
    nx double precision,
    ny double precision,
    nz double precision
)
RETURNS geometry AS $$
    SELECT ST_RotateY(ST_RotateX(geom, atan2(ny, nz)), -atan2(nx, sqrt(ny*ny + nz*nz)));
$$ LANGUAGE sql IMMUTABLE STRICT;

-- Inverse of face_to_plane: places a 2D geometry of the rotated frame at height z
-- and rotates it back into world coordinates.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.face_from_plane(
    geom geometry,
    z double precision,
    nx double precision,
    ny double precision,
    nz double precision
)
RETURNS geometry AS $$
    SELECT ST_RotateX(
        ST_RotateY(ST_Force3DZ(geom, z), atan2(nx, sqrt(ny*ny + nz*nz))),
        -atan2(ny, nz)
    );
$$ LANGUAGE sql IMMUTABLE STRICT;
