-- Finds the LoD surface features (RoofSurface, WallSurface, GroundSurface) belonging to
-- each building by following the CityDB `boundary` property from the feature that owns
-- the lodNSolid (the Building, or its BuildingPart), then taking each surface's own
-- lodNMultiSurface geometry. Only MULTIPOLYGON geometries are collected; script 02
-- explodes these into individual polygon faces. Skips buildings already present in
-- _child_feature. building_object_id and surface_object_id are captured here from
-- {lod_schema}.feature so they propagate through all downstream tables without a JOIN.
--
-- The `lodNMultiSurface` filter matters for 3DBAG: a BuildingPart carries a `boundary`
-- row for every surface at every LoD it ships (1.2, 1.3, 2.2), so without the filter the
-- LoD1.3 box surfaces attach alongside the LoD2.2 ones. For single-LoD datasets
-- (DE, AT) the filter is a no-op.

WITH buildings AS (
  SELECT DISTINCT f.id AS building_feature_id, f.objectid AS building_object_id
  FROM {lod_schema}.feature f
  JOIN {lod_schema}.property p ON f.id = p.feature_id
    AND p.name = 'lod' || {lod_level} || 'Solid'
  WHERE f.objectclass_id BETWEEN 900 AND 999
    AND f.id NOT IN (
      SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature
    ) -- Exclude already processed buildings
    AND f.id IN {building_ids}
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature (
    id,
    lod,
    building_feature_id,
    surface_feature_id,
    building_object_id,
    surface_object_id,
    objectclass_id,
    classname,
    geom
)
SELECT
    gen_random_uuid(),
    {lod_level},
    b.building_feature_id,
    sf.id AS surface_feature_id,
    b.building_object_id,
    sf.objectid AS surface_object_id,
    sf.objectclass_id,
    oc.classname,
    g.geometry AS geometry
FROM buildings b
JOIN {lod_schema}.property boundary_link ON boundary_link.feature_id = b.building_feature_id
  AND boundary_link.name = 'boundary'
JOIN {lod_schema}.feature sf ON sf.id = boundary_link.val_feature_id
JOIN {lod_schema}.objectclass oc ON oc.id = sf.objectclass_id
JOIN {lod_schema}.property surface_geom ON surface_geom.feature_id = sf.id
  AND surface_geom.name = 'lod' || {lod_level} || 'MultiSurface'
JOIN {lod_schema}.geometry_data g ON g.id = surface_geom.val_geometry_id
WHERE sf.objectclass_id NOT BETWEEN 900 AND 999
  AND GeometryType(g.geometry) = 'MULTIPOLYGON';
