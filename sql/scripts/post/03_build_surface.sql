-- Builds {lod}_surface from {lod}_surface_raw, one row per served polygon piece: the exterior
-- pieces of each face and, flagged is_party_wall, its shared pieces. Runs once over the whole
-- table after party-wall detection and replaces every row.

TRUNCATE {city2tabula_schema}.{lod_schema}_surface;

WITH pieces AS (
    SELECT sr.id AS raw_id, FALSE AS party, d.geom,
           (sr.geom_envelope IS NULL AND sr.geom_exposed IS NULL) AS whole_face
    FROM {city2tabula_schema}.{lod_schema}_surface_raw sr
    CROSS JOIN LATERAL ST_Dump(COALESCE(sr.geom_envelope, sr.geom_exposed, ST_Multi(sr.geom))) d
    UNION ALL
    SELECT sr.id, TRUE, d.geom, FALSE
    FROM {city2tabula_schema}.{lod_schema}_surface_raw sr
    CROSS JOIN LATERAL ST_Dump(sr.geom_party) d
    WHERE sr.geom_party IS NOT NULL
)
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
    neighbour_object_id,
    geom
)
SELECT
    sr.building_object_id,
    sr.surface_object_id,
    sr.surface_feature_id,
    sr.classname AS surface_type,
    a.surface_area,
    (a.surface_area IS NOT NULL AND a.surface_area <= 0) AS area_below_precision,
    sr.tilt,
    sr.azimuth,
    CASE WHEN p.whole_face THEN sr.height
         ELSE ROUND((ST_ZMax(p.geom) - ST_ZMin(p.geom))::numeric, 2) END AS height,
    CASE WHEN p.whole_face THEN sr.length ELSE ROUND(d.length::numeric, 2) END AS length,
    CASE WHEN p.whole_face THEN sr.width ELSE ROUND(d.width::numeric, 2) END AS width,
    sr.is_valid,
    sr.is_planar,
    p.party AS is_party_wall,
    CASE WHEN p.party THEN sr.neighbour_object_id END AS neighbour_object_id,
    p.geom
FROM pieces p
JOIN {city2tabula_schema}.{lod_schema}_surface_raw sr ON sr.id = p.raw_id
CROSS JOIN LATERAL (
    SELECT CASE WHEN p.whole_face THEN sr.surface_area
                ELSE ROUND(ST_Area(ST_Force2D({city2tabula_schema}.face_to_plane(
                         p.geom, sr.normal_x, sr.normal_y, sr.normal_z)))::numeric, 2)
           END AS surface_area
) a
LEFT JOIN LATERAL {city2tabula_schema}.surface_dimensions(p.geom, sr.normal_x, sr.normal_y, sr.normal_z) d
  ON NOT p.whole_face
WHERE sr.building_object_id IS NOT NULL
  AND sr.surface_object_id IS NOT NULL
  -- Script 04 gives no row to a building with ground faces only; serve no surfaces for it.
  AND EXISTS (
      SELECT 1 FROM {city2tabula_schema}.{lod_schema}_building b
      WHERE b.building_feature_id = sr.building_feature_id
  );
