WITH buildings AS (
  SELECT f.id AS building_feature_id, f.objectid AS building_objectid, g.geometry AS building_geom
  FROM {lod_schema}.feature f
  JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
  JOIN {lod_schema}.property p ON f.id = p.feature_id
    AND p.name = 'lod' || {lod_level} || 'Solid'
  WHERE objectclass_id  BETWEEN 900 AND 999 AND f.id NOT IN (
    SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature_raw
  ) -- Exclude already processed buildings
  AND f.id IN {building_ids}
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_raw (
    id,
    lod,
    building_feature_id,
    building_objectid,
    surface_feature_id,
    surface_objectid,
    objectclass_id,
    classname,
    geom
)
SELECT
    gen_random_uuid(),
    {lod_level},
    b.building_feature_id,
    b.building_objectid,
    f.id AS surface_feature_id,
    f.objectid AS surface_objectid,
    f.objectclass_id,
    oc.classname,
    g.geometry AS geometry
FROM {lod_schema}.feature f
JOIN {lod_schema}.objectclass oc ON f.objectclass_id = oc.id
JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
JOIN buildings b ON ST_3DIntersects(g.geometry, b.building_geom)
WHERE f.objectclass_id NOT BETWEEN 900 AND 999
  AND f.id != b.building_feature_id
  AND GeometryType(g.geometry) IN ('MULTIPOLYGON');
