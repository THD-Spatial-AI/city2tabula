WITH
-- Buildings in this batch: ground boundary in 2D (metres, EPSG:25832)
b AS (
  SELECT
    building_feature_id,
    ST_Boundary(ST_Force2D(geom)) AS ground_bnd_2d
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
),

-- Wall geometry: one row per wall_id (use stage-3 surface geometry)
w AS (
  SELECT DISTINCT ON (s.surface_feature_id)
    s.surface_feature_id AS wall_id,
    ST_Boundary(ST_Force2D(s.geom)) AS wall_bnd_2d
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

-- For each wall, pick the 2 nearest buildings by boundary distance (KNN).
-- NOTE: KNN is fast only if ground_bnd_2d is indexed in a real table; in a CTE it still works
-- but may not be as fast. This is still much cheaper than intersections.
nearest2 AS (
  SELECT
    w.wall_id,
    nb.building_feature_id,
    nb.dist_m,
    ROW_NUMBER() OVER (PARTITION BY w.wall_id ORDER BY nb.dist_m ASC, nb.building_feature_id) AS rn
  FROM w
  JOIN LATERAL (
    SELECT
      b.building_feature_id,
      ST_Distance(w.wall_bnd_2d, b.ground_bnd_2d) AS dist_m
    FROM b
    ORDER BY w.wall_bnd_2d <-> b.ground_bnd_2d
    LIMIT 2
  ) nb ON TRUE
),

picked AS (
  SELECT
    n1.wall_id,
    n1.building_feature_id AS building_feature_id,
    n1.dist_m AS best_dist_m,
    n2.building_feature_id AS second_building_id,
    n2.dist_m AS second_dist_m
  FROM nearest2 n1
  LEFT JOIN nearest2 n2
    ON n2.wall_id = n1.wall_id AND n2.rn = 2
  WHERE n1.rn = 1
),

final_walls AS (
  SELECT
    p.wall_id,
    p.building_feature_id,
    p.best_dist_m,
    'wall_nearest_ground_boundary_2d'::varchar AS scoring_method,

    -- Party-wall heuristic:
    -- mark as party wall if 2nd nearest is within 0.30 m of the best (tune if needed)
    (p.second_building_id IS NOT NULL AND (p.second_dist_m - p.best_dist_m) <= 0.30) AS is_party_wall,
    CASE
      WHEN (p.second_building_id IS NOT NULL AND (p.second_dist_m - p.best_dist_m) <= 0.30)
        THEN p.second_building_id
      ELSE NULL
    END AS party_with_building_id
  FROM picked p

  -- Hard safety gate: if even the nearest building is too far, don't “resolve” it.
  -- This avoids nonsense assignments when data is broken.
  WHERE p.best_dist_m <= 2.00   -- metres; tune (1–3m typical)
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

  -- Score: store distance (smaller is better). If your pipeline expects "higher is better",
  -- replace with: 1.0 / (fw.best_dist_m + 1e-6)
  fw.best_dist_m AS score,

  fw.scoring_method,
  fw.is_party_wall,
  fw.party_with_building_id,

  -- pick a deterministic geom for the wall (do not filter by building_feature_id)
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