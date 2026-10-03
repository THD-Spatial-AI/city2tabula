-- Calculates surface area, tilt, azimuth, height, length and width for each surface polygon.
--
-- Pipeline stages:
--   1. new_buildings -> skip already-processed buildings
--   2. owner_interior_pts -> one interior point per solid (from GroundSurface)
--   3. raw_surfaces -> one row per polygon face
--   4. surface_points -> explode polygon to 3D vertices (ST_DumpPoints)
--   5. surface_edges -> pair each vertex with its LEAD successor in its ring
--   6. surface_normals -> Newell's method: accumulate edge-pair cross-products
--   7. oriented_normals -> dot-product flip to enforce outward-facing normal
--   8. normalized_normals -> per-class flip rules + unit normalisation
--   9. convergence_corrected -> placeholder for UTM meridian convergence (see Discussion)
--   10. INSERT -> _surface_raw (length/width via surface_dimensions)
--   11. UPDATE -> area_internal / geom_exposed for faces between solids of one building
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
      FROM {city2tabula_schema}.{lod_schema}_surface_raw
    )
),

owner_interior_pts AS (
  -- ST_PointOnSurface guarantees a point inside the polygon even for non-convex
  -- footprints (L-shaped, U-shaped) where ST_Centroid can fall outside.
  -- Taken per solid, not per building: inside a building of several parts, a
  -- point in one part would flip the outward test for walls of another.
  -- ST_Collect (not ST_Union) aggregates without dissolving, avoiding lock
  -- contention during parallel batch processing.
  -- LEFT-joined downstream so buildings without a GroundSurface still proceed.
  SELECT
    owner_feature_id,
    ST_PointOnSurface(ST_Collect(ST_Force2D(geom))) AS interior_pt
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE building_feature_id IN (SELECT building_feature_id FROM new_buildings)
    AND classname = 'GroundSurface'
  GROUP BY owner_feature_id
),

raw_surfaces AS (
  -- is_planar is carried to the output column only; Newell's method handles
  -- planar and non-planar surfaces uniformly so no routing is needed here.
  SELECT
    gd.id,
    gd.child_row_id,
    gd.building_feature_id,
    gd.owner_feature_id,
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
    (ST_DumpPoints(valid_geom)).path[1] AS ring_idx,
    (ST_DumpPoints(valid_geom)).path[2] AS pt_idx
  FROM raw_surfaces
),

surface_edges AS (
  -- LEAD pairs each vertex with its successor in the same ring; the NULL for a
  -- ring's last row is filtered in surface_normals. The closing vertex (duplicate
  -- of the first) provides the final edge back to the ring start automatically.
  -- pt_idx restarts in every ring, so a face with holes needs the ring partition.
  SELECT
    id, child_row_id, building_feature_id, owner_feature_id, surface_feature_id,
    building_object_id, surface_object_id,
    objectclass_id, classname, valid_geom, is_planar,
    point_geom,
    LEAD(point_geom) OVER (PARTITION BY id, ring_idx ORDER BY pt_idx) AS next_pt
  FROM surface_points
),

surface_normals AS (
  -- HAVING COUNT(*) >= 2 requires at least two valid edge pairs.
  -- The outer WHERE discards degenerate surfaces (all vertices collinear →
  -- zero cross-product magnitude → no well-defined normal).
  SELECT
    id, child_row_id, building_feature_id, owner_feature_id, surface_feature_id,
    building_object_id, surface_object_id,
    objectclass_id, classname, valid_geom, is_planar,
    n_x, n_y, n_z,
    sqrt(n_x * n_x + n_y * n_y + n_z * n_z) AS cross_magnitude
  FROM (
    SELECT
      id, child_row_id, building_feature_id, owner_feature_id, surface_feature_id,
      building_object_id, surface_object_id,
      objectclass_id, classname, valid_geom, is_planar,
      SUM((ST_Y(point_geom) - ST_Y(next_pt)) * (ST_Z(point_geom) + ST_Z(next_pt))) AS n_x,
      SUM((ST_Z(point_geom) - ST_Z(next_pt)) * (ST_X(point_geom) + ST_X(next_pt))) AS n_y,
      SUM((ST_X(point_geom) - ST_X(next_pt)) * (ST_Y(point_geom) + ST_Y(next_pt))) AS n_z
    FROM surface_edges
    WHERE next_pt IS NOT NULL
    GROUP BY id, child_row_id, building_feature_id, owner_feature_id, surface_feature_id,
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
  LEFT JOIN owner_interior_pts bp ON bp.owner_feature_id = n.owner_feature_id
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
    owner_feature_id,
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

INSERT INTO {city2tabula_schema}.{lod_schema}_surface_raw (
    id,
    building_feature_id,
    owner_feature_id,
    surface_feature_id,
    building_object_id,
    surface_object_id,
    objectclass_id,
    classname,
    normal_x,
    normal_y,
    normal_z,
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
    length,
    length_unit,
    width,
    width_unit,
    geom
)
SELECT
    gen_random_uuid() AS id,
    building_feature_id,
    owner_feature_id,
    surface_feature_id,
    building_object_id,
    surface_object_id,
    objectclass_id,
    classname,
    nx,
    ny,
    nz,
    -- Self-intersecting geometries give net signed area via the shoelace formula
    -- (crossing sub-regions cancel). ST_MakeValid decomposes them into valid
    -- sub-polygons so ST_Area sums correctly. Called only for invalid surfaces;
    -- ST_IsValid is CSE'd with the is_valid column below (one evaluation per row).
    -- Rounded to 2 decimals (cm-level for lengths, cm² for areas) at the point of
    -- computation — matches the ±0.04 m/° accuracy already validated against
    -- reference data, so further digits are float noise, not real precision.
    ROUND(
      (CASE
        WHEN objectclass_id IN (709, 710, 712)
          THEN {city2tabula_schema}.surface_area_corrected_geom(valid_geom, nx, ny, nz)
        ELSE NULL
      END)::numeric, 2
    ) AS surface_area,
    'sqm' AS surface_area_unit,
    -- ASIN(|nz|): 0° for vertical wall (nz=0), 90° for flat roof (|nz|=1). This
    -- is the complement of the usual from-horizontal slope angle, so a downstream
    -- consumer converts with 90 - tilt.
    -- Snapped to 90 (flat) when |nz| > 0.985, the same near-horizontal band
    -- where azimuth below is undefined, rather than reporting the raw near-90
    -- value from that numerically unstable region.
    CASE
      WHEN ABS(nz) > 0.985 THEN 90.0
      ELSE ROUND(DEGREES(ASIN(ABS(nz)))::numeric, 2)
    END AS tilt,
    'degrees' AS tilt_unit,
    -- (450 − atan2(ny, nx)) mod 360 converts math convention (CCW from east)
    -- to compass convention (CW from grid north). Suppressed (−1) when |nz| > 0.985
    -- (tilt > ~80°) where horizontal components are near zero and atan2 is unstable.
    -- convergence_deg corrects grid north → geographic north (currently 0°; see Discussion).
    CASE
      WHEN ABS(nz) > 0.985 THEN -1
      ELSE ROUND(
        MOD(
          MOD((450.0 - degrees(atan2(ny::numeric, nx::numeric)))::numeric + 360.0, 360.0)
          + convergence_deg::numeric + 360.0,
          360.0
        ), 2
      )
    END AS azimuth,
    'degrees' AS azimuth_unit,
    ST_IsValid(valid_geom) AS is_valid,
    is_planar,
    child_row_id,
    ROUND((ST_ZMax(valid_geom) - ST_ZMin(valid_geom))::numeric, 2) AS height,
    'm',
    ROUND(d.length::numeric, 2) AS length,
    'm',
    ROUND(d.width::numeric, 2) AS width,
    'm',
    valid_geom AS geom
FROM convergence_corrected
-- LATERAL evaluates the function once per row; same classes as surface_area.
LEFT JOIN LATERAL {city2tabula_schema}.surface_dimensions(valid_geom, nx, ny, nz) d
  ON objectclass_id IN (709, 710, 712);

-- Internal faces. Where two solids of one building touch, the face region between
-- them is interior: a source that models each part as a closed solid (Vienna) has
-- it on both parts, a source that leaves the parts open on that side (Prague) has
-- it on neither. A face is internal where a face of another solid of the same
-- building lies on it:
--   * normals parallel within ~2.6 degrees (|cos| >= 0.999), between two walls or
--     between a roof and a ground. The sign of the wall normals is not used: the
--     outward flip above points some walls of concave footprints inward, and two
--     solids cannot both have exterior wall on the same patch in any case;
--   * plane offset <= 0.05 m, read as a Z difference in the face_to_plane frame;
--   * overlap = 2D intersection in that plane.
-- The exposed remainder keeps only pieces wider than 0.04 m and larger than
-- 0.01 m2, so the tolerance band leaves no slivers. Faces between different
-- buildings (party walls) are not considered here.
WITH faces AS (
  SELECT id, building_feature_id, owner_feature_id, classname,
         normal_x AS nx, normal_y AS ny, normal_z AS nz, geom
  FROM {city2tabula_schema}.{lod_schema}_surface_raw
  WHERE building_feature_id IN {building_ids}
    AND normal_x IS NOT NULL
    AND classname IN ('WallSurface', 'RoofSurface', 'GroundSurface')
),
covered AS (
  SELECT a.id,
         ST_Union(ST_CollectionExtract(ST_Intersection(
           ST_ReducePrecision(ST_MakeValid(ST_Force2D(ra.g)), 0.001),
           ST_ReducePrecision(ST_MakeValid(ST_Force2D(rb.g)), 0.001)
         ), 3)) AS geom_2d
  FROM faces a
  JOIN faces b
    ON b.building_feature_id = a.building_feature_id
   AND b.owner_feature_id <> a.owner_feature_id
   AND ABS(a.nx * b.nx + a.ny * b.ny + a.nz * b.nz) >= 0.999
   AND (
         (a.classname = 'WallSurface' AND b.classname = 'WallSurface')
         OR (a.classname <> b.classname AND 'WallSurface' NOT IN (a.classname, b.classname))
       )
   AND ST_3DDWithin(a.geom, b.geom, 0.05)
  CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(a.geom, a.nx, a.ny, a.nz) AS g) ra
  CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(b.geom, a.nx, a.ny, a.nz) AS g) rb
  WHERE ABS((ST_ZMin(rb.g) + ST_ZMax(rb.g)) - (ST_ZMin(ra.g) + ST_ZMax(ra.g))) / 2 <= 0.05
  GROUP BY a.id
),
exposed AS (
  SELECT
    f.id, f.nx, f.ny, f.nz,
    (ST_ZMin(r.g) + ST_ZMax(r.g)) / 2 AS z,
    (SELECT ST_Collect(d.geom)
     FROM ST_Dump(ST_Buffer(ST_Buffer(
            ST_Difference(ST_ReducePrecision(ST_MakeValid(ST_Force2D(r.g)), 0.001), c.geom_2d),
            -0.02, 'join=mitre'), 0.02, 'join=mitre')) d
     WHERE ST_Area(d.geom) >= 0.01) AS geom_2d
  FROM covered c
  JOIN faces f ON f.id = c.id
  CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(f.geom, f.nx, f.ny, f.nz) AS g) r
  WHERE ST_Area(c.geom_2d) >= 0.01
)
UPDATE {city2tabula_schema}.{lod_schema}_surface_raw sr
SET
  area_internal = GREATEST(ROUND((sr.surface_area - COALESCE(ST_Area(e.geom_2d), 0))::numeric, 2), 0),
  geom_exposed = COALESCE(
    ST_Multi({city2tabula_schema}.face_from_plane(e.geom_2d, e.z, e.nx, e.ny, e.nz)),
    ST_SetSRID('MULTIPOLYGON Z EMPTY'::geometry, {srid})
  )
FROM exposed e
WHERE sr.id = e.id;
