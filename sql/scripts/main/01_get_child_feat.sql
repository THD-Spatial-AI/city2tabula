WITH buildings AS (
  SELECT f.id AS building_feature_id, g.geometry AS building_geom
  FROM {lod_schema}.feature f
  JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
  WHERE f.objectclass_id IN (901, 905) AND f.id NOT IN (
    SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature
  ) -- Exclude already processed buildings
  AND f.id IN {building_ids}
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature (
    id,
    lod,
    building_feature_id,
    surface_feature_id,
    objectclass_id,
    classname,
    geom
)
SELECT
    gen_random_uuid(),
    {lod_level},
    b.building_feature_id,
    f.id AS surface_feature_id,
    f.objectclass_id,
    oc.classname,
    g.geometry AS geometry
FROM {lod_schema}.feature f
JOIN {lod_schema}.objectclass oc ON f.objectclass_id = oc.id
JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
JOIN buildings b ON ST_3DIntersects(g.geometry, b.building_geom)
WHERE f.objectclass_id NOT IN (901, 905)
  AND f.id != b.building_feature_id
  AND GeometryType(g.geometry) IN ('MULTIPOLYGON', 'POLYHEDRALSURFACE');

-- JOIN buildings b ON
--   CASE
--     WHEN ST_GeometryType(g.geometry) = 'ST_PolyhedralSurface' OR ST_GeometryType(b.building_geom) = 'ST_PolyhedralSurface'
--     THEN ST_3DIntersects(g.geometry, b.building_geom)
--     ELSE ST_Intersects(ST_Force2D(g.geometry), ST_Force2D(b.building_geom))
--   END

-- JOIN buildings b ON ST_3DIntersects(g.geometry, b.building_geom)
