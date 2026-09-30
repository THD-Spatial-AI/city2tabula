-- Populates lod2_surface from lod2_surface_raw: one row per exposed surface polygon
-- patch, with party walls excluded.
--
-- lod2_surface_raw already holds one row per polygon face (script 02 explodes each
-- surface feature's MultiSurface via ST_Dump). A single surface feature can carry
-- several faces: a 3DBAG WallSurface is one feature covering every wall of the
-- building. All faces are carried through here; collapsing to one row per
-- surface_feature_id / surface_object_id would drop most of the geometry.
--
-- building_object_id and surface_object_id are captured in script 01 and carried
-- through the intermediate tables, so no JOIN back to the source schema is needed.
--
-- Faces between two solids of one building (script 03) are served as their exposed
-- remainder: a fully internal face yields no row, a partly internal face one row per
-- piece of geom_exposed, with area, height, length and width measured on the piece.
-- Tilt and azimuth are the parent face's, since a piece lies in the same plane.
--
-- Buildings already present in lod2_surface are skipped, so re-running is safe.
--
-- Party-wall exclusion: is_party_wall is set by a neighbour-detection step that is
-- not currently wired into the pipeline, so it stays NULL/FALSE and every non-party
-- surface passes. Re-run this script after neighbour detection to apply exclusions.

INSERT INTO {city2tabula_schema}.{lod_schema}_surface (
    building_object_id,
    surface_object_id,
    surface_feature_id,
    surface_type,
    surface_area,
    area_below_precision,
    tilt,
    azimuth,
    height,
    length,
    width,
    is_valid,
    is_planar,
    is_party_wall,
    neighbour_building_id,
    geom
)
SELECT
    sr.building_object_id,
    sr.surface_object_id,
    sr.surface_feature_id,
    sr.classname        AS surface_type,
    a.surface_area,
    (a.surface_area IS NOT NULL AND a.surface_area <= 0) AS area_below_precision,
    sr.tilt,
    sr.azimuth,
    CASE WHEN sr.geom_exposed IS NULL THEN sr.height
         ELSE ROUND((ST_ZMax(piece.geom) - ST_ZMin(piece.geom))::numeric, 2) END AS height,
    CASE WHEN sr.geom_exposed IS NULL THEN sr.length ELSE ROUND(d.length::numeric, 2) END AS length,
    CASE WHEN sr.geom_exposed IS NULL THEN sr.width ELSE ROUND(d.width::numeric, 2) END AS width,
    sr.is_valid,
    sr.is_planar,
    sr.is_party_wall,
    sr.neighbour_building_id,
    piece.geom
FROM {city2tabula_schema}.{lod_schema}_surface_raw sr
CROSS JOIN LATERAL ST_Dump(COALESCE(sr.geom_exposed, ST_Multi(sr.geom))) piece
CROSS JOIN LATERAL (
    SELECT CASE WHEN sr.geom_exposed IS NULL THEN sr.surface_area
                ELSE ROUND(ST_Area(ST_Force2D({city2tabula_schema}.face_to_plane(
                         piece.geom, sr.normal_x, sr.normal_y, sr.normal_z)))::numeric, 2)
           END AS surface_area
) a
LEFT JOIN LATERAL {city2tabula_schema}.surface_dimensions(piece.geom, sr.normal_x, sr.normal_y, sr.normal_z) d
  ON sr.geom_exposed IS NOT NULL
WHERE sr.building_feature_id IN {building_ids}
  AND sr.building_object_id IS NOT NULL
  AND sr.surface_object_id  IS NOT NULL
  AND (sr.is_party_wall IS NULL OR sr.is_party_wall = FALSE)
  AND sr.building_object_id NOT IN (
      SELECT s.building_object_id
      FROM {city2tabula_schema}.{lod_schema}_surface s
  );
