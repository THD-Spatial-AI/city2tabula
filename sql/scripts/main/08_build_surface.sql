-- Populates lod2_surface from lod2_surface_raw.
--
-- building_object_id and surface_object_id are captured in script 01 and
-- carried through all intermediate tables, so no JOIN back to the source
-- schema is needed here.
--
-- Party-wall surfaces are excluded. Party-wall flags are set by the neighbour
-- detection pipeline (-detect-neighbours). If that pipeline has not run yet,
-- is_party_wall is NULL and all surfaces are included. Re-run this script
-- after neighbour detection to apply party-wall exclusions.

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
  AND (sr.is_party_wall IS NULL OR sr.is_party_wall = FALSE)
  AND sr.building_object_id IS NOT NULL
  AND sr.surface_object_id  IS NOT NULL
ON CONFLICT DO NOTHING;
