-- Populates lod2_surface from lod2_surface_raw: one row per surface polygon patch,
-- with party walls excluded.
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
    tilt,
    azimuth,
    height,
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
    sr.surface_area,
    sr.tilt,
    sr.azimuth,
    sr.height,
    sr.is_valid,
    sr.is_planar,
    sr.is_party_wall,
    sr.neighbour_building_id,
    sr.geom
FROM {city2tabula_schema}.{lod_schema}_surface_raw sr
WHERE sr.building_feature_id IN {building_ids}
  AND sr.building_object_id IS NOT NULL
  AND sr.surface_object_id  IS NOT NULL
  AND (sr.is_party_wall IS NULL OR sr.is_party_wall = FALSE)
  AND sr.building_object_id NOT IN (
      SELECT s.building_object_id
      FROM {city2tabula_schema}.{lod_schema}_surface s
  );
