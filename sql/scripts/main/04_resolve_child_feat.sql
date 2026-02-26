WITH walls AS (
  SELECT building_feature_id, surface_feature_id, geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
  WHERE classname = 'WallSurface'
    AND building_feature_id IN {building_ids}
),
grounds AS (
  SELECT building_feature_id, surface_feature_id, geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
  WHERE classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
),

-- 2D shared-boundary-length scoring (robust against non-planar polygons)
pairs AS (
  SELECT
    g.building_feature_id,
    g.surface_feature_id AS ground_id,
    -- boundary length in XY (works even if Z is messy)
    ST_Length(
      ST_Intersection(
        ST_Boundary(ST_Buffer(ST_Force2D(g.geom), 0)),
        ST_Boundary(ST_Buffer(ST_Force2D(w.geom), 0))
      )
    ) AS shared_len
  FROM grounds g
  JOIN walls w
    ON w.building_feature_id = g.building_feature_id
   AND ST_Intersects(ST_Force2D(g.geom), ST_Force2D(w.geom))
),

scores AS (
  SELECT
    building_feature_id,
    ground_id,
    SUM(shared_len) AS score_len
  FROM pairs
  GROUP BY building_feature_id, ground_id
  HAVING SUM(shared_len) > 0
),

ranked AS (
  SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY building_feature_id ORDER BY score_len DESC) AS rn
  FROM scores
),

first_pick AS (
  SELECT * FROM ranked WHERE rn = 1
),

-- keep strongest claimant per ground_id (prevents duplicates)
first_pick_dedup AS (
  SELECT DISTINCT ON (ground_id) *
  FROM first_pick
  ORDER BY ground_id, score_len DESC
),

dropped_buildings AS (
  SELECT fp.building_feature_id
  FROM first_pick fp
  LEFT JOIN first_pick_dedup k
    ON k.building_feature_id = fp.building_feature_id
   AND k.ground_id = fp.ground_id
  WHERE k.building_feature_id IS NULL
),

second_try AS (
  SELECT r.*
  FROM ranked r
  JOIN dropped_buildings d ON d.building_feature_id = r.building_feature_id
  WHERE r.rn > 1
),

taken_grounds AS (
  SELECT ground_id FROM first_pick_dedup
),

promoted AS (
  SELECT DISTINCT ON (building_feature_id) st.*
  FROM second_try st
  LEFT JOIN taken_grounds tg ON tg.ground_id = st.ground_id
  WHERE tg.ground_id IS NULL
  ORDER BY building_feature_id, rn
),

winner_table AS (
  SELECT * FROM first_pick_dedup
  UNION ALL
  SELECT * FROM promoted
)

INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_resolved (
  lod,
  surface_feature_id,
  building_feature_id,
  objectclass_id,
  classname,
  score,
  scoring_method,
  geom
)
SELECT
  {lod_level} AS lod,
  w.ground_id AS surface_feature_id,
  w.building_feature_id,
  710 AS objectclass_id,
  'GroundSurface' AS classname,
  w.score_len AS score,
  'ground_wall_shared_edge_len_2d' AS scoring_method,
  c.geom
FROM winner_table w
JOIN {city2tabula_schema}.{lod_schema}_child_feature c
  ON c.lod = {lod_level}
 AND c.building_feature_id = w.building_feature_id
 AND c.surface_feature_id = w.ground_id

ON CONFLICT (lod, surface_feature_id) DO UPDATE
SET building_feature_id = EXCLUDED.building_feature_id,
    objectclass_id      = EXCLUDED.objectclass_id,
    classname           = EXCLUDED.classname,
    score               = EXCLUDED.score,
    scoring_method      = EXCLUDED.scoring_method,
    geom                = EXCLUDED.geom;