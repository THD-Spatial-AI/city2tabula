-- Resolves stable source objectids for buildings and their surfaces, then:
--   1. Updates lod2_building_feature.object_id from lod2.feature.objectid
--   2. Populates lod2_surface_link — the stable resolved building→surface mapping
--
-- Run order: after all seven feature-extraction and labelling scripts.
-- The session-local integer IDs (building_feature_id, surface_feature_id) are
-- still valid at this point and used as the join key back to lod2.feature.
--
-- Why this is a separate final step (not wired into scripts 01–07):
--   Scripts 01–07 use integer IDs as fast join keys; carrying VARCHAR objectids
--   through every intermediate CTE and temp table would add overhead with no
--   benefit during the computation phase. Resolving once at the end is cheaper.
--
-- lod2_surface_link only captures non-party-wall surfaces (is_party_wall IS NULL
-- or FALSE). Party walls are an artefact of the spatial intersection algorithm
-- and not physical surface ownership; they must not appear in the stable link.

-- Step 1: backfill object_id in lod2_building_feature
UPDATE {city2tabula_schema}.{lod_schema}_building_feature bf
SET object_id = f.objectid
FROM {lod_schema}.feature f
WHERE bf.building_feature_id = f.id
  AND bf.object_id IS NULL;

-- Step 2: populate the surface link table
INSERT INTO {city2tabula_schema}.{lod_schema}_surface_link
    (building_object_id, surface_object_id, surface_type)
SELECT
    bf_feat.objectid  AS building_object_id,
    sf_feat.objectid  AS surface_object_id,
    cfs.classname     AS surface_type
FROM {city2tabula_schema}.{lod_schema}_child_feature_surface cfs
JOIN {lod_schema}.feature bf_feat
    ON bf_feat.id = cfs.building_feature_id
JOIN {lod_schema}.feature sf_feat
    ON sf_feat.id = cfs.surface_feature_id
WHERE cfs.building_feature_id IN {building_ids}
  AND (cfs.is_party_wall IS NULL OR cfs.is_party_wall = FALSE)
  AND bf_feat.objectid IS NOT NULL
  AND sf_feat.objectid IS NOT NULL
ON CONFLICT DO NOTHING;
