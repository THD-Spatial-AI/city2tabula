WITH
-- 1) Roof ids involved in this batch (from stage-1 mapping)
roof_ids AS (
  SELECT DISTINCT c.surface_feature_id AS roof_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  WHERE c.lod = {lod_level}
    AND c.classname = 'RoofSurface'
    AND c.building_feature_id IN {building_ids}
),

-- 2) Candidate (roof -> building) links from dirty mapping (DEDUPED)
roof_candidates AS (
  SELECT DISTINCT c.surface_feature_id AS roof_id, c.building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  JOIN roof_ids r ON r.roof_id = c.surface_feature_id
  WHERE c.lod = {lod_level}
    AND c.classname = 'RoofSurface'
    AND c.building_feature_id IN {building_ids}
),

-- 3) Ground polygons for candidate buildings (2D)
b AS (
  SELECT
    building_feature_id,
    ST_Buffer(ST_Force2D(geom), 0) AS ground_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
),

-- 4) Roof polygons (2D) + original 3D geom
r AS (
  SELECT DISTINCT ON (s.surface_feature_id)
    s.surface_feature_id AS roof_id,
    ST_Buffer(ST_Force2D(s.geom), 0) AS roof_2d,
    s.geom AS roof_geom_3d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_surface s
  JOIN roof_ids ids ON ids.roof_id = s.surface_feature_id
  WHERE s.classname = 'RoofSurface'
  ORDER BY s.surface_feature_id
),

-- 5) Score only candidate pairs (roof -> buildings that claim it)
scores AS (
  SELECT
    rc.roof_id,
    rc.building_feature_id,
    ST_Area(ST_Intersection(r.roof_2d, b.ground_2d)) AS overlap_m2
  FROM roof_candidates rc
  JOIN r ON r.roof_id = rc.roof_id
  JOIN b ON b.building_feature_id = rc.building_feature_id
  WHERE r.roof_2d && b.ground_2d
    AND ST_Intersects(r.roof_2d, b.ground_2d)
),

-- 6) Pick best building per roof
ranked AS (
  SELECT
    s.*,
    ROW_NUMBER() OVER (
      PARTITION BY s.roof_id
      ORDER BY s.overlap_m2 DESC, s.building_feature_id
    ) AS rn
  FROM scores s
),

picked AS (
  SELECT
    roof_id,
    building_feature_id,
    overlap_m2
  FROM ranked
  WHERE rn = 1
),

-- 7) Fallback for roofs with no score at all (keep deterministic owner from mapping)
fallback AS (
  SELECT
    rc.roof_id,
    MIN(rc.building_feature_id) AS building_feature_id,
    0.0::double precision AS overlap_m2
  FROM roof_candidates rc
  WHERE NOT EXISTS (
    SELECT 1 FROM scores s WHERE s.roof_id = rc.roof_id
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
  708 AS objectclass_id,  -- VERIFY in your CityDB; change if needed
  'RoofSurface' AS classname,
  fr.overlap_m2 AS score,
  'roof_ground_overlap_area_2d_candidates'::varchar AS scoring_method,
  FALSE AS is_party_wall,
  NULL::bigint AS party_with_building_id,
  r.roof_geom_3d AS geom
FROM final_roofs fr
JOIN r ON r.roof_id = fr.roof_id
ORDER BY fr.roof_id, fr.overlap_m2 DESC
ON CONFLICT (lod, surface_feature_id) DO UPDATE
SET building_feature_id    = EXCLUDED.building_feature_id,
    objectclass_id         = EXCLUDED.objectclass_id,
    classname              = EXCLUDED.classname,
    score                  = EXCLUDED.score,
    scoring_method         = EXCLUDED.scoring_method,
    is_party_wall          = EXCLUDED.is_party_wall,
    party_with_building_id = EXCLUDED.party_with_building_id,
    geom                   = EXCLUDED.geom;