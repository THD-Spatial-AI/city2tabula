-- Builds the stable building→surface mapping in lod2_surface_link.
--
-- building_object_id and surface_object_id are captured in script 01 from
-- lod2.feature and carried through all intermediate tables, so no JOIN back
-- to the source schema is needed here.
--
-- Party-wall surfaces are excluded. Party-wall flags are set by the neighbour
-- detection pipeline (-detect-neighbours). If that pipeline has not run yet,
-- is_party_wall is NULL and all surfaces are included — re-run this script
-- after neighbour detection to get the final filtered link table.

INSERT INTO {city2tabula_schema}.{lod_schema}_surface_link
    (building_object_id, surface_object_id, surface_type)
SELECT
    cfs.building_object_id,
    cfs.surface_object_id,
    cfs.classname AS surface_type
FROM {city2tabula_schema}.{lod_schema}_child_feature_surface cfs
WHERE cfs.building_feature_id IN {building_ids}
  AND (cfs.is_party_wall IS NULL OR cfs.is_party_wall = FALSE)
  AND cfs.building_object_id IS NOT NULL
  AND cfs.surface_object_id  IS NOT NULL
ON CONFLICT DO NOTHING;
