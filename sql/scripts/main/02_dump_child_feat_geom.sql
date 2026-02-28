WITH new_buildings AS (
  -- Select only buildings that haven't been processed yet
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature_raw
  WHERE building_feature_id IN {building_ids}
    AND building_feature_id NOT IN (
      SELECT DISTINCT building_feature_id
      FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
    )
),
dumped AS (
  SELECT
    c.id AS child_row_id,
    c.building_feature_id,
    c.building_objectid,
    c.surface_feature_id,
    c.surface_objectid,
    c.objectclass_id,
    c.classname AS classname,
    (ST_Dump(c.geom)).geom AS geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_raw c
  INNER JOIN new_buildings nb ON c.building_feature_id = nb.building_feature_id
)
INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (
    id,
    child_row_id,
    building_feature_id,
    building_objectid,
    surface_feature_id,
    surface_objectid,
    objectclass_id,
    classname,
    coord_dim,
    has_z,
    geom
)
SELECT
    gen_random_uuid() AS id,
    child_row_id,
    building_feature_id,
    building_objectid,
    surface_feature_id,
    surface_objectid,
    objectclass_id,
    classname,
    ST_CoordDim(geom) AS coord_dim,
    (ST_ZMin(geom) IS NOT NULL) AS has_z,
    geom::geometry(POLYGONZ) AS geom
FROM dumped;