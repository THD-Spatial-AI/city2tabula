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
first3_points AS (
  SELECT
    id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    ARRAY_AGG(point_geom ORDER BY pt_idx) FILTER (WHERE pt_idx <= 3) AS pts
  FROM points
  GROUP BY id, child_row_id, building_feature_id, surface_feature_id, objectclass_id, classname, valid_geom
  HAVING COUNT(*) >= 3
),
normals AS (
  SELECT *,
    pts[1] AS p1,
    pts[2] AS p2,
    pts[3] AS p3,
    ST_X(pts[2]) - ST_X(pts[1]) AS a_x,
    ST_Y(pts[2]) - ST_Y(pts[1]) AS a_y,
    ST_Z(pts[2]) - ST_Z(pts[1]) AS a_z,
    ST_X(pts[3]) - ST_X(pts[1]) AS b_x,
    ST_Y(pts[3]) - ST_Y(pts[1]) AS b_y,
    ST_Z(pts[3]) - ST_Z(pts[1]) AS b_z
  FROM first3_points
),
computed AS (
  SELECT *,
    (a_y * b_z - a_z * b_y) AS n_x,
    (a_z * b_x - a_x * b_z) AS n_y,
    (a_x * b_y - a_y * b_x) AS n_z
  FROM normals
),
final AS (
  SELECT *,
    sqrt(n_x^2 + n_y^2 + n_z^2) AS norm_len,
    n_x / NULLIF(sqrt(n_x^2 + n_y^2 + n_z^2), 0) AS nx,
    n_y / NULLIF(sqrt(n_x^2 + n_y^2 + n_z^2), 0) AS ny,
    n_z / NULLIF(sqrt(n_x^2 + n_y^2 + n_z^2), 0) AS nz
  FROM computed
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
    (90 - degrees(acos(nz))) AS tilt,
    'degrees' AS tilt_unit,
    CASE
      WHEN ABS(nz) > 0.99 THEN -1
      ELSE MOD((450.0 - degrees(atan2(ny::numeric, nx::numeric)))::numeric, 360.0)
    END AS azimuth,
    'degrees' AS azimuth_unit,
    ST_IsValid(valid_geom) AS is_valid,
    true AS is_planar,
    child_row_id,
      (ST_ZMax(valid_geom) - ST_ZMin(valid_geom)) AS height,
    'm',
    valid_geom AS geom
FROM final;