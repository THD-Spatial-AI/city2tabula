-- =============================================================================
-- Wall Surface Resolver  —  "Sandwich" approach (optimised)
-- =============================================================================
-- Runs AFTER ground (03) and roof (04) have already been resolved.
--
-- Strategy:
--   1. Use fast KNN distance to ground boundary for candidate selection
--      and scoring (same speed as the original resolver).
--   2. Use cheap ST_Intersects booleans to classify each candidate into
--      a tier based on whether the wall touches the building's resolved
--      ground AND/OR roof (the "sandwich" check).
--   3. Rank by tier first (sandwich > ground-only > roof-only), then by
--      distance — so sandwich matches always win, but the per-row cost
--      stays close to the old distance-only implementation.
--
-- Tier priority:
--   1 = wall touches BOTH ground and roof  (sandwich — highest confidence)
--   2 = wall touches only the ground boundary
--   3 = wall touches only the roof boundary
-- =============================================================================

WITH

-- 1) Resolved ground polygons per building (2D, for KNN + intersects check)
ground AS (
  SELECT
    building_feature_id,
    ST_Force2D(geom) AS ground_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
),

-- 2) Resolved roof polygons per building (2D, kept per-surface — no ST_Union)
roof AS (
  SELECT
    building_feature_id,
    ST_Force2D(geom) AS roof_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND classname = 'RoofSurface'
    AND building_feature_id IN {building_ids}
),

-- 3) Wall geometry: one row per wall_id (2D polygon, cheap)
w AS (
  SELECT DISTINCT ON (s.surface_feature_id)
    s.surface_feature_id AS wall_id,
    ST_Force2D(s.geom) AS wall_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump s
  WHERE s.classname = 'WallSurface'
    AND s.surface_feature_id IN (
      SELECT DISTINCT c.surface_feature_id
      FROM {city2tabula_schema}.{lod_schema}_child_feature c
      WHERE c.lod = {lod_level}
        AND c.classname = 'WallSurface'
        AND c.building_feature_id IN {building_ids}
    )
  ORDER BY s.surface_feature_id
),

-- 4) For each wall, pick nearest 2 buildings by ground-boundary distance (fast KNN).
--    Then check cheaply whether the wall also intersects any roof of that building.
nearest2 AS (
  SELECT
    w.wall_id,
    w.wall_2d,
    nb.building_feature_id,
    nb.dist_m,
    nb.touches_ground,
    -- Check if wall intersects ANY resolved roof of this building (cheap EXISTS)
    EXISTS (
      SELECT 1 FROM roof r
      WHERE r.building_feature_id = nb.building_feature_id
        AND ST_Intersects(w.wall_2d, r.roof_2d)
    ) AS touches_roof,
    ROW_NUMBER() OVER (
      PARTITION BY w.wall_id
      ORDER BY nb.dist_m ASC, nb.building_feature_id
    ) AS rn
  FROM w
  JOIN LATERAL (
    SELECT
      g.building_feature_id,
      ST_Distance(
        ST_Boundary(w.wall_2d),
        ST_Boundary(g.ground_2d)
      ) AS dist_m,
      ST_Intersects(w.wall_2d, g.ground_2d) AS touches_ground
    FROM ground g
    ORDER BY ST_Boundary(w.wall_2d) <-> ST_Boundary(g.ground_2d)
    LIMIT 2
  ) nb ON TRUE
),

-- 5) Assign tier + score
scored AS (
  SELECT
    wall_id,
    building_feature_id,
    dist_m,
    touches_ground,
    touches_roof,
    CASE
      WHEN touches_ground AND touches_roof THEN 1  -- sandwich
      WHEN touches_ground                  THEN 2  -- ground only
      WHEN touches_roof                    THEN 3  -- roof only
      ELSE 4
    END AS match_tier,
    rn
  FROM nearest2
  WHERE touches_ground OR touches_roof
),

-- 6) Rank: prefer sandwich, then shortest distance
ranked AS (
  SELECT
    *,
    ROW_NUMBER() OVER (
      PARTITION BY wall_id
      ORDER BY match_tier ASC,
               dist_m ASC,
               building_feature_id
    ) AS pick_rn
  FROM scored
),

-- 7) Pick best + party-wall detection
picked AS (
  SELECT
    r1.wall_id,
    r1.building_feature_id,
    r1.dist_m       AS best_dist_m,
    r1.match_tier,
    r2.building_feature_id AS second_building_id,
    r2.dist_m              AS second_dist_m,
    r2.match_tier          AS second_tier
  FROM ranked r1
  LEFT JOIN ranked r2 ON r2.wall_id = r1.wall_id AND r2.pick_rn = 2
  WHERE r1.pick_rn = 1
),

final_walls AS (
  SELECT
    p.wall_id,
    p.building_feature_id,
    p.best_dist_m,
    ('wall_sandwich_ground_roof_2d_tier' || p.match_tier)::varchar AS scoring_method,

    -- Party-wall heuristic:
    -- Both top-2 are sandwich (tier 1), and 2nd distance within 0.30 m of best
    (p.second_building_id IS NOT NULL
     AND p.match_tier = 1 AND p.second_tier = 1
     AND (p.second_dist_m - p.best_dist_m) <= 0.30) AS is_party_wall,
    CASE
      WHEN (p.second_building_id IS NOT NULL
            AND p.match_tier = 1 AND p.second_tier = 1
            AND (p.second_dist_m - p.best_dist_m) <= 0.30)
        THEN p.second_building_id
      ELSE NULL
    END AS party_with_building_id
  FROM picked p
  -- Safety gate: don't resolve walls whose nearest match is too far
  WHERE p.best_dist_m <= 2.00   -- metres; tune (1–3 m typical)
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
SELECT
  {lod_level} AS lod,
  fw.wall_id AS surface_feature_id,
  fw.building_feature_id,
  s.building_objectid,
  s.surface_objectid,
  709 AS objectclass_id,
  'WallSurface' AS classname,

  -- Score: ground distance (smaller = better), same unit as original resolver
  fw.best_dist_m AS score,

  fw.scoring_method,
  fw.is_party_wall,
  fw.party_with_building_id,

  -- pick a deterministic geom for the wall
  s.geom
FROM final_walls fw
JOIN LATERAL (
  SELECT s.building_objectid, s.surface_objectid, s.geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump s
  WHERE s.classname = 'WallSurface'
    AND s.surface_feature_id = fw.wall_id
  ORDER BY s.surface_feature_id
  LIMIT 1
) s ON TRUE

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
