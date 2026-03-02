WITH
-- 1) Roof ids involved in this batch (from stage-1 mapping)
roof_ids AS MATERIALIZED (
  SELECT DISTINCT c.surface_feature_id AS roof_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  WHERE c.lod = {lod_level}
    AND c.classname = 'RoofSurface'
    AND c.building_feature_id IN {building_ids}
),

-- 2) Candidate (roof -> building) links from dirty mapping (DEDUPED)
roof_candidates AS MATERIALIZED (
  SELECT DISTINCT c.surface_feature_id AS roof_id, c.building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  JOIN roof_ids ri ON ri.roof_id = c.surface_feature_id
  WHERE c.lod = {lod_level}
    AND c.classname = 'RoofSurface'
    AND c.building_feature_id IN {building_ids}
),

-- 3) Ground polygons for candidate buildings (2D, no ST_Buffer repair)
b AS MATERIALIZED (
  SELECT
    building_feature_id,
    ST_Force2D(geom) AS ground_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
),

-- 4) Roof polygons (2D) + original 3D geom (no ST_Buffer repair)
r AS MATERIALIZED (
  SELECT DISTINCT ON (s.surface_feature_id)
    s.surface_feature_id AS roof_id,
    s.building_objectid,
    s.surface_objectid,
    ST_Force2D(s.geom) AS roof_2d,
    s.geom AS roof_geom_3d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump s
  JOIN roof_ids ids ON ids.roof_id = s.surface_feature_id
  WHERE s.classname = 'RoofSurface'
  ORDER BY s.surface_feature_id
),

-- 5) For each roof, pick nearest 2 candidate buildings by KNN distance,
--    then check overlap with a cheap ST_Intersects boolean.
scored AS (
  SELECT
    r.roof_id,
    nb.building_feature_id,
    nb.dist_m,
    nb.overlaps_ground
  FROM r
  JOIN LATERAL (
    SELECT
      rc.building_feature_id,
      ST_Distance(r.roof_2d, b.ground_2d) AS dist_m,
      ST_Intersects(r.roof_2d, b.ground_2d) AS overlaps_ground
    FROM roof_candidates rc
    JOIN b ON b.building_feature_id = rc.building_feature_id
    WHERE rc.roof_id = r.roof_id
    ORDER BY r.roof_2d <-> b.ground_2d
    LIMIT 2
  ) nb ON TRUE
),

-- 6) Rank: prefer overlapping candidates, then closest distance
ranked AS (
  SELECT
    s.*,
    ROW_NUMBER() OVER (
      PARTITION BY s.roof_id
      ORDER BY
        (NOT s.overlaps_ground)::int ASC,   -- overlapping first
        s.dist_m ASC,
        s.building_feature_id
    ) AS rn
  FROM scored s
),

picked AS (
  SELECT
    roof_id,
    building_feature_id,
    dist_m
  FROM ranked
  WHERE rn = 1
),

-- 7) Fallback for roofs with no candidates at all
fallback AS (
  SELECT
    rc.roof_id,
    MIN(rc.building_feature_id) AS building_feature_id,
    -1.0::double precision AS dist_m
  FROM roof_candidates rc
  WHERE NOT EXISTS (
    SELECT 1 FROM scored s WHERE s.roof_id = rc.roof_id
  )
  GROUP BY rc.roof_id
),

final_roofs AS (
  SELECT * FROM picked
  UNION ALL
  SELECT * FROM fallback
)

INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_resolved (
  lod,
  surface_feature_id,
  building_feature_id,
  building_objectid,
  surface_objectid,
  objectclass_id,
  classname,
  score,
  scoring_method,
  is_party_wall,
  party_with_building_id,
  geom
)
SELECT DISTINCT ON (fr.roof_id)
  {lod_level} AS lod,
  fr.roof_id AS surface_feature_id,
  fr.building_feature_id,
  r.building_objectid,
  r.surface_objectid,
  712 AS objectclass_id,
  'RoofSurface' AS classname,
  fr.dist_m AS score,
  'roof_ground_knn_2d'::varchar AS scoring_method,
  FALSE AS is_party_wall,
  NULL::bigint AS party_with_building_id,
  r.roof_geom_3d AS geom
FROM final_roofs fr
JOIN r ON r.roof_id = fr.roof_id
ORDER BY fr.roof_id, fr.dist_m ASC
ON CONFLICT (lod, surface_feature_id) DO UPDATE
SET building_feature_id    = EXCLUDED.building_feature_id,
    building_objectid       = EXCLUDED.building_objectid,
    surface_objectid        = EXCLUDED.surface_objectid,
    objectclass_id         = EXCLUDED.objectclass_id,
    classname              = EXCLUDED.classname,
    score                  = EXCLUDED.score,
    scoring_method         = EXCLUDED.scoring_method,
    is_party_wall          = EXCLUDED.is_party_wall,
    party_with_building_id = EXCLUDED.party_with_building_id,
    geom                   = EXCLUDED.geom;
