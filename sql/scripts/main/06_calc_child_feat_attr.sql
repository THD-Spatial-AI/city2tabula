-- Calculate surface attributes (surface_area, tilt, azimuth, height)
-- for resolved surfaces only, then INSERT into child_feature_surface
-- as the final clean table.
--
-- Flow: resolved table (ownership) + geom_dump (polygon parts) → attributes → child_feature_surface

WITH resolved AS MATERIALIZED (
  SELECT surface_feature_id, building_feature_id, classname, objectclass_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND building_feature_id IN {building_ids}
),

-- Get all polygon parts for resolved surfaces from the geometry dump.
-- Join on BOTH columns so we naturally deduplicate across dirty building mappings.
target_rows AS (
  SELECT
    gd.id AS dump_id,
    gd.child_row_id,
    r.building_feature_id,
    gd.surface_feature_id,
    r.objectclass_id,
    r.classname,
    gd.geom AS valid_geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump gd
  INNER JOIN resolved r
    ON gd.surface_feature_id = r.surface_feature_id
   AND gd.building_feature_id = r.building_feature_id
),

points AS (
  SELECT
    *,
    (ST_DumpPoints(valid_geom)).geom AS point_geom,
    (ST_DumpPoints(valid_geom)).path[2] AS pt_idx
  FROM target_rows
),

all_points AS (
  SELECT
    dump_id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    ARRAY_AGG(point_geom ORDER BY pt_idx) AS all_pts
  FROM points
  GROUP BY dump_id, child_row_id, building_feature_id, surface_feature_id,
           objectclass_id, classname, valid_geom
  HAVING COUNT(*) >= 3
),

-- Find the first three non-collinear points
point_combinations AS (
  SELECT
    dump_id,
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

-- Pick first valid (non-collinear) triplet per surface polygon
normals AS (
  SELECT DISTINCT ON (dump_id, child_row_id, building_feature_id, surface_feature_id)
    dump_id,
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
  WHERE cross_magnitude > 1e-10
  ORDER BY dump_id, child_row_id, building_feature_id, surface_feature_id, i, j, k
),

-- Normalise and flip roof normals upward
final AS (
  SELECT
    dump_id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
      THEN -(n_x / NULLIF(cross_magnitude, 0))
      ELSE n_x / NULLIF(cross_magnitude, 0)
    END AS nx,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
      THEN -(n_y / NULLIF(cross_magnitude, 0))
      ELSE n_y / NULLIF(cross_magnitude, 0)
    END AS ny,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
      THEN -(n_z / NULLIF(cross_magnitude, 0))
      ELSE n_z / NULLIF(cross_magnitude, 0)
    END AS nz
  FROM normals
),

-- Derive all scalar attributes in one pass
computed AS (
  SELECT
    dump_id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    valid_geom,
    CASE
      WHEN objectclass_id IN (709, 710, 712)
        THEN {city2tabula_schema}.surface_area_corrected_geom(valid_geom, nx, ny, nz)
      ELSE NULL
    END AS surface_area,
    DEGREES(ASIN(ABS(nz))) AS tilt,
    CASE
      WHEN ABS(nz) < 0.01 THEN -1  -- vertical surface, azimuth undefined
      ELSE MOD((450.0 - degrees(atan2(ny::numeric, nx::numeric)))::numeric + 360.0, 360.0)
    END AS azimuth,
    ST_IsPlanar(valid_geom) AS is_planar,
    (ST_ZMax(valid_geom) - ST_ZMin(valid_geom)) AS height
  FROM final
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
    attribute_calc_status,
    geom
)
SELECT
    gen_random_uuid() AS id,
    c.building_feature_id,
    c.surface_feature_id,
    c.objectclass_id,
    c.classname,
    c.surface_area,
    'sqm' AS surface_area_unit,
    c.tilt,
    'degrees' AS tilt_unit,
    c.azimuth,
    'degrees' AS azimuth_unit,
    ST_IsValid(c.valid_geom) AS is_valid,
    c.is_planar,
    c.child_row_id,
    c.height,
    'm' AS height_unit,
    'resolved' AS attribute_calc_status,
    c.valid_geom AS geom
FROM computed c;
