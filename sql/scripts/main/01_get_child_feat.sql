-- Finds the LoD surface features (RoofSurface, WallSurface, GroundSurface) belonging to
-- each Building. A Building's geometry is every lodNSolid owned by the Building itself
-- or by any BuildingPart below it (reached through the `buildingPart` property, at any
-- depth). For each solid owner, the CityDB `boundary` property leads to its surfaces,
-- and each surface's own lodNMultiSurface geometry is taken. Only MULTIPOLYGON
-- geometries are collected; script 02 explodes these into individual polygon faces.
-- Skips buildings already present in _child_feature.
--
-- building_feature_id / building_object_id are always the Building's, so every part
-- aggregates into one output row keyed by the Building's own object_id.
-- owner_feature_id is the solid owner, which scripts 03 and 04 need to orient faces,
-- find the faces between parts, and take heights per part. Sources differ only in
-- which features own solids: the Building alone (Bremen), one part (3DBAG), several
-- parts (Vienna), or the Building and its parts (Prague).
--
-- BuildingInstallation surfaces hang off the `buildingInstallation` property, which is
-- not followed: an installation (dormer, chimney, balcony) sits on a closed envelope
-- and adding its surfaces would count that envelope twice.
--
-- The `lodNMultiSurface` filter matters for 3DBAG: a BuildingPart carries a `boundary`
-- row for every surface at every LoD it ships (1.2, 1.3, 2.2), so without the filter the
-- LoD1.3 box surfaces attach alongside the LoD2.2 ones. For single-LoD datasets
-- (DE, AT) the filter is a no-op.

WITH RECURSIVE members AS (
  SELECT f.id AS building_feature_id, f.objectid AS building_object_id, f.id AS member_id
  FROM {lod_schema}.feature f
  WHERE f.objectclass_id = 901
    AND f.id IN {building_ids}
    AND f.id NOT IN (
      SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature
    ) -- Exclude already processed buildings
  UNION ALL
  SELECT m.building_feature_id, m.building_object_id, p.val_feature_id
  FROM members m
  JOIN {lod_schema}.property p ON p.feature_id = m.member_id
    AND p.name = 'buildingPart'
),
owners AS (
  SELECT DISTINCT m.building_feature_id, m.building_object_id, m.member_id AS owner_feature_id
  FROM members m
  JOIN {lod_schema}.property s ON s.feature_id = m.member_id
    AND s.name = 'lod' || {lod_level} || 'Solid'
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature (
    id,
    lod,
    building_feature_id,
    owner_feature_id,
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
    o.building_feature_id,
    o.owner_feature_id,
    sf.id AS surface_feature_id,
    o.building_object_id,
    sf.objectid AS surface_object_id,
    sf.objectclass_id,
    oc.classname,
    g.geometry AS geometry
FROM owners o
JOIN {lod_schema}.property boundary_link ON boundary_link.feature_id = o.owner_feature_id
  AND boundary_link.name = 'boundary'
JOIN {lod_schema}.feature sf ON sf.id = boundary_link.val_feature_id
JOIN {lod_schema}.objectclass oc ON oc.id = sf.objectclass_id
JOIN {lod_schema}.property surface_geom ON surface_geom.feature_id = sf.id
  AND surface_geom.name = 'lod' || {lod_level} || 'MultiSurface'
JOIN {lod_schema}.geometry_data g ON g.id = surface_geom.val_geometry_id
WHERE sf.objectclass_id NOT BETWEEN 900 AND 999
  AND GeometryType(g.geometry) = 'MULTIPOLYGON';
