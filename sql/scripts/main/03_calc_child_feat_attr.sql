-- This script calculates surface area, tilt, and azimuth.

WITH new_buildings AS (
  -- Select only buildings that haven't been processed yet
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE building_feature_id IN {building_ids}
    AND building_feature_id NOT IN (
      SELECT DISTINCT building_feature_id
      FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
    )
),
dumped AS (
  SELECT
    gd.id,
    gd.child_row_id,
    gd.building_feature_id,
    gd.surface_feature_id,
    gd.objectclass_id,
    gd.classname,
    gd.geom AS valid_geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump gd
  INNER JOIN new_buildings nb ON gd.building_feature_id = nb.building_feature_id
),
points AS (
  SELECT
    *,
    (ST_DumpPoints(valid_geom)).geom AS point_geom,
    (ST_DumpPoints(valid_geom)).path[2] AS pt_idx
  FROM dumped
),
all_points AS (
  SELECT
    id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    ARRAY_AGG(point_geom ORDER BY pt_idx) AS all_pts
  FROM points
  GROUP BY id, child_row_id, building_feature_id, surface_feature_id, objectclass_id, classname, valid_geom
  HAVING COUNT(*) >= 3
),
-- Find the first three non-colinear points by testing combinations
point_combinations AS (
  SELECT
    id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    all_pts[i] AS p1,
    all_pts[j] AS p2,
    all_pts[k] AS p3,
    i, j, k
  FROM all_points,
    LATERAL generate_series(1, LEAST(array_length(all_pts, 1), 10)) i,
    LATERAL generate_series(i + 1, LEAST(array_length(all_pts, 1), 10)) j,
    LATERAL generate_series(j + 1, LEAST(array_length(all_pts, 1), 10)) k
),
vectors AS (
  SELECT *,
    ST_X(p2) - ST_X(p1) AS a_x,
    ST_Y(p2) - ST_Y(p1) AS a_y,
    ST_Z(p2) - ST_Z(p1) AS a_z,
    ST_X(p3) - ST_X(p1) AS b_x,
    ST_Y(p3) - ST_Y(p1) AS b_y,
    ST_Z(p3) - ST_Z(p1) AS b_z
  FROM point_combinations
),
cross_products AS (
  SELECT *,
    (a_y * b_z - a_z * b_y) AS n_x,
    (a_z * b_x - a_x * b_z) AS n_y,
    (a_x * b_y - a_y * b_x) AS n_z,
    sqrt((a_y * b_z - a_z * b_y)^2 +
         (a_z * b_x - a_x * b_z)^2 +
         (a_x * b_y - a_y * b_x)^2) AS cross_magnitude
  FROM vectors
),
-- Select the first valid (non-colinear) combination for each surface
normals AS (
  SELECT DISTINCT ON (id, child_row_id, building_feature_id, surface_feature_id)
    id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    p1, p2, p3,
    a_x, a_y, a_z,
    b_x, b_y, b_z,
    n_x, n_y, n_z,
    cross_magnitude
  FROM cross_products
  WHERE cross_magnitude > 1e-10  -- Filter out colinear points
  ORDER BY id, child_row_id, building_feature_id, surface_feature_id, i, j, k
),
-- Calculate tilt and azimuth
-- Tilt = angle between normal vector and vertical axis
-- Azimuth = compass direction of the projection of the normal vector onto the horizontal plane
-- Using formulas:
-- tilt = DEGREES(ASIN(ABS(n_z) / |n|))
-- azimuth = MOD((DEGREES(ATAN2(n_x, n_y)) + 360), 360)
final AS (
  SELECT *,
    cross_magnitude AS norm_len,
    n_x / NULLIF(cross_magnitude, 0) AS nx,
    n_y / NULLIF(cross_magnitude, 0) AS ny,
    n_z / NULLIF(cross_magnitude, 0) AS nz
  FROM normals
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_surface (
    id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    surface_area,
    surface_area_unit,
    tilt,
    tilt_unit,
    azimuth,
    azimuth_unit,
    is_valid,
    is_planar,
    child_row_id,
    height,
    height_unit,
    geom
)
SELECT
    gen_random_uuid() AS id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    CASE
      WHEN objectclass_id IN (709, 710, 712)
        THEN {city2tabula_schema}.surface_area_corrected_geom(valid_geom, nx, ny, nz)
      ELSE NULL
    END AS surface_area,
    'sqm' AS surface_area_unit,
    DEGREES(ASIN(ABS(nz))) AS tilt,
    'degrees' AS tilt_unit,
    CASE
      WHEN ABS(nz) > 0.99 THEN -1
      ELSE MOD((450.0 - degrees(atan2(ny::numeric, nx::numeric)))::numeric + 360.0, 360.0)
    END AS azimuth,
    'degrees' AS azimuth_unit,
    ST_IsValid(valid_geom) AS is_valid,
    true AS is_planar,
    child_row_id,
      (ST_ZMax(valid_geom) - ST_ZMin(valid_geom)) AS height,
    'm',
    valid_geom AS geom
FROM final;