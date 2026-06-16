-- Calculates surface area, tilt, azimuth, and height for each surface polygon.
--
-- Pipeline stages:
--   1. new_buildings -> skip already-processed buildings
--   2. building_interior_pts -> one interior point per building (from GroundSurface)
--   3. raw_surfaces -> one row per polygon face
--   4. surface_points -> explode polygon to 3D vertices (ST_DumpPoints)
--   5. surface_edges -> pair each vertex with its LEAD successor
--   6. surface_normals -> Newell's method: accumulate edge-pair cross-products
--   7. oriented_normals -> dot-product flip to enforce outward-facing normal
--   8. normalized_normals -> per-class flip rules + unit normalisation
--   9. convergence_corrected -> placeholder for UTM meridian convergence (see Discussion)
--   10. INSERT -> _child_feature_surface
--
-- Newell's method (surface_normals):
--   nx = SUM (y_i − y_{i+1}) * (z_i + z_{i+1})
--   ny = SUM (z_i − z_{i+1}) * (x_i + x_{i+1})
--   nz = SUM (x_i − x_{i+1}) * (y_i + y_{i+1})
--   Handles non-planar polygons correctly (best-fit normal); planar polygons give
--   the same result as any 3-point cross-product. O(n) per surface.

WITH new_buildings AS (
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE building_feature_id IN {building_ids}
    AND building_feature_id NOT IN (
      SELECT DISTINCT building_feature_id
      FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
    )
),

building_interior_pts AS (
  -- ST_PointOnSurface guarantees a point inside the polygon even for non-convex
  -- footprints (L-shaped, U-shaped) where ST_Centroid can fall outside.
  -- ST_Collect (not ST_Union) aggregates without dissolving, avoiding lock
  -- contention during parallel batch processing.
  -- LEFT-joined downstream so buildings without a GroundSurface still proceed.
  SELECT
    building_feature_id,
    ST_PointOnSurface(ST_Collect(ST_Force2D(geom))) AS interior_pt
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE building_feature_id IN (SELECT building_feature_id FROM new_buildings)
    AND classname = 'GroundSurface'
  GROUP BY building_feature_id
),

raw_surfaces AS (
  -- is_planar is carried to the output column only; Newell's method handles
  -- planar and non-planar surfaces uniformly so no routing is needed here.
  SELECT
    gd.id,
    gd.child_row_id,
    gd.building_feature_id,
    gd.surface_feature_id,
    gd.building_object_id,
    gd.surface_object_id,
    gd.objectclass_id,
    gd.classname,
    gd.geom AS valid_geom,
    ST_IsPlanar(gd.geom) AS is_planar
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump gd
  INNER JOIN new_buildings nb ON gd.building_feature_id = nb.building_feature_id
),

surface_points AS (
  SELECT
    *,
    (ST_DumpPoints(valid_geom)).geom AS point_geom,
    (ST_DumpPoints(valid_geom)).path[2] AS pt_idx
  FROM raw_surfaces
),

surface_edges AS (
  -- LEAD pairs each vertex with its successor; the NULL for the last row is
  -- filtered in surface_normals. The closing vertex (duplicate of the first)
  -- provides the final edge back to the ring start automatically.
  SELECT
    id, child_row_id, building_feature_id, surface_feature_id,
    building_object_id, surface_object_id,
    objectclass_id, classname, valid_geom, is_planar,
    point_geom,
    LEAD(point_geom) OVER (PARTITION BY id ORDER BY pt_idx) AS next_pt
  FROM surface_points
),

surface_normals AS (
  -- HAVING COUNT(*) >= 2 requires at least two valid edge pairs.
  -- The outer WHERE discards degenerate surfaces (all vertices collinear →
  -- zero cross-product magnitude → no well-defined normal).
  SELECT
    id, child_row_id, building_feature_id, surface_feature_id,
    building_object_id, surface_object_id,
    objectclass_id, classname, valid_geom, is_planar,
    n_x, n_y, n_z,
    sqrt(n_x * n_x + n_y * n_y + n_z * n_z) AS cross_magnitude
  FROM (
    SELECT
      id, child_row_id, building_feature_id, surface_feature_id,
      building_object_id, surface_object_id,
      objectclass_id, classname, valid_geom, is_planar,
      SUM((ST_Y(point_geom) - ST_Y(next_pt)) * (ST_Z(point_geom) + ST_Z(next_pt))) AS n_x,
      SUM((ST_Z(point_geom) - ST_Z(next_pt)) * (ST_X(point_geom) + ST_X(next_pt))) AS n_y,
      SUM((ST_X(point_geom) - ST_X(next_pt)) * (ST_Y(point_geom) + ST_Y(next_pt))) AS n_z
    FROM surface_edges
    WHERE next_pt IS NOT NULL
    GROUP BY id, child_row_id, building_feature_id, surface_feature_id,
             building_object_id, surface_object_id,
             objectclass_id, classname, valid_geom, is_planar
    HAVING COUNT(*) >= 2
  ) sums
  WHERE sqrt(n_x * n_x + n_y * n_y + n_z * n_z) > 1e-10
),

oriented_normals AS (
  -- Dot product of (centroid − interior_pt) with (nx, ny) determines whether
  -- the horizontal normal points outward (≥ 0) or inward (< 0).
  -- This corrects CW/CCW vertex winding differences across CityGML datasets:
  -- opposite winding flips the cross-product 180°, swapping north↔south walls.
  -- COALESCE to +1 when no GroundSurface interior point exists.
  SELECT
    n.*,
    COALESCE(
      CASE
        WHEN (
            (ST_X(ST_Centroid(ST_Force2D(n.valid_geom))) - ST_X(bp.interior_pt))
              * (n.n_x / NULLIF(n.cross_magnitude, 0))
          + (ST_Y(ST_Centroid(ST_Force2D(n.valid_geom))) - ST_Y(bp.interior_pt))
              * (n.n_y / NULLIF(n.cross_magnitude, 0))
        ) >= 0 THEN 1.0
        ELSE -1.0
      END,
      1.0
    ) AS surface_flip
  FROM surface_normals n
  LEFT JOIN building_interior_pts bp ON bp.building_feature_id = n.building_feature_id
),

normalized_normals AS (
  -- RoofSurface: flip when nz < 0 to guarantee upward-facing normal.
  --   nz sign is reliable for any non-flat roof; the dot-product test is NOT
  --   used here because nx/ny are near-zero for low-tilt roofs (sign-unstable).
  -- WallSurface: apply surface_flip to enforce outward-facing horizontal normal.
  -- Other types: unit normal as-is.
  SELECT
    id,
    child_row_id,
    building_feature_id,
    surface_feature_id,
    building_object_id,
    surface_object_id,
    objectclass_id,
    classname,
    valid_geom,
    is_planar,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
        THEN -(n_x / NULLIF(cross_magnitude, 0))
      WHEN classname = 'WallSurface'
        THEN surface_flip * (n_x / NULLIF(cross_magnitude, 0))
      ELSE n_x / NULLIF(cross_magnitude, 0)
    END AS nx,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
        THEN -(n_y / NULLIF(cross_magnitude, 0))
      WHEN classname = 'WallSurface'
        THEN surface_flip * (n_y / NULLIF(cross_magnitude, 0))
      ELSE n_y / NULLIF(cross_magnitude, 0)
    END AS ny,
    CASE
      WHEN classname = 'RoofSurface' AND (n_z / NULLIF(cross_magnitude, 0)) < 0
        THEN -(n_z / NULLIF(cross_magnitude, 0))
      WHEN classname = 'WallSurface'
        THEN surface_flip * (n_z / NULLIF(cross_magnitude, 0))
      ELSE n_z / NULLIF(cross_magnitude, 0)
    END AS nz
  FROM oriented_normals
),

convergence_corrected AS (
  SELECT nn.*, 0.0 AS convergence_deg
  FROM normalized_normals nn
)

INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_surface (
    id,
    building_feature_id,
    surface_feature_id,
    building_object_id,
    surface_object_id,
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
    building_object_id,
    surface_object_id,
    objectclass_id,
    classname,
    -- Self-intersecting geometries give net signed area via the shoelace formula
    -- (crossing sub-regions cancel). ST_MakeValid decomposes them into valid
    -- sub-polygons so ST_Area sums correctly. Called only for invalid surfaces;
    -- ST_IsValid is CSE'd with the is_valid column below (one evaluation per row).
    CASE
      WHEN objectclass_id IN (709, 710, 712)
        THEN {city2tabula_schema}.surface_area_corrected_geom(valid_geom, nx, ny, nz)
      ELSE NULL
    END AS surface_area,
    'sqm' AS surface_area_unit,
    -- ASIN(|nz|): 0° for vertical wall (nz=0), 90° for flat roof (|nz|=1).
    DEGREES(ASIN(ABS(nz))) AS tilt,
    'degrees' AS tilt_unit,
    -- (450 − atan2(ny, nx)) mod 360 converts math convention (CCW from east)
    -- to compass convention (CW from grid north). Suppressed (−1) when |nz| > 0.985
    -- (tilt > ~80°) where horizontal components are near zero and atan2 is unstable.
    -- convergence_deg corrects grid north → geographic north (currently 0°; see Discussion).
    CASE
      WHEN ABS(nz) > 0.985 THEN -1
      ELSE MOD(
        MOD((450.0 - degrees(atan2(ny::numeric, nx::numeric)))::numeric + 360.0, 360.0)
        + convergence_deg::numeric + 360.0,
        360.0
      )
    END AS azimuth,
    'degrees' AS azimuth_unit,
    ST_IsValid(valid_geom) AS is_valid,
    is_planar,
    child_row_id,
    (ST_ZMax(valid_geom) - ST_ZMin(valid_geom)) AS height,
    'm',
    valid_geom AS geom
FROM convergence_corrected;
