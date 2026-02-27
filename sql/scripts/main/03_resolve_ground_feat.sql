WITH
-- classify buildings by how many ground surfaces they have
ground_counts AS (
  SELECT
    building_feature_id,
    COUNT(*) AS ground_cnt
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE classname = 'GroundSurface'
    AND building_feature_id IN {building_ids}
  GROUP BY building_feature_id
),

multi_ground_buildings AS (
  SELECT building_feature_id
  FROM ground_counts
  WHERE ground_cnt > 1
),

single_ground AS (
  SELECT
    s.building_feature_id,
    s.surface_feature_id AS ground_id,
    s.building_objectid,
    s.surface_objectid,
    s.geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump s
  JOIN ground_counts gc
    ON gc.building_feature_id = s.building_feature_id
  WHERE s.classname = 'GroundSurface'
    AND s.building_feature_id IN {building_ids}
    AND gc.ground_cnt = 1
),

walls AS (
  SELECT building_feature_id, surface_feature_id, building_objectid, surface_objectid, geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE classname = 'WallSurface'
    AND building_feature_id IN (SELECT building_feature_id FROM multi_ground_buildings)
),

grounds AS (
  SELECT building_feature_id, surface_feature_id, building_objectid, surface_objectid, geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
  WHERE classname = 'GroundSurface'
    AND building_feature_id IN (SELECT building_feature_id FROM multi_ground_buildings)
),

-- 2D shared-boundary-length scoring
pairs AS (
  SELECT
    g.building_feature_id,
    g.surface_feature_id AS ground_id,
    g.building_objectid,
    g.surface_objectid,
      -- compute shared boundary length in 2D (ignore Z to be more robust to minor vertical misalignments)
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

-- keep strongest claimant per ground_id (prevents duplicates across buildings)
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
  JOIN dropped_buildings d
    ON d.building_feature_id = r.building_feature_id
  WHERE r.rn > 1
),

taken_grounds AS (
  SELECT ground_id FROM first_pick_dedup
),

promoted AS (
  SELECT DISTINCT ON (building_feature_id) st.*
  FROM second_try st
  LEFT JOIN taken_grounds tg
    ON tg.ground_id = st.ground_id
  WHERE tg.ground_id IS NULL
  ORDER BY building_feature_id, rn
),

winner_table AS (
  SELECT * FROM first_pick_dedup
  UNION ALL
  SELECT * FROM promoted
),

-- any multi-ground building that produced no winner (e.g., score_len never > 0)
unresolved_multi_grounds AS (
  SELECT g.building_feature_id, g.surface_feature_id AS ground_id, g.building_objectid, g.surface_objectid, g.geom
  FROM grounds g
  LEFT JOIN winner_table w
    ON w.building_feature_id = g.building_feature_id
   AND w.ground_id = g.surface_feature_id
  WHERE w.ground_id IS NULL
),

-- everything we will insert: winners + single-ground buildings + unresolved leftovers
to_insert AS (
  -- winners (have score)
  SELECT
    w.building_feature_id,
    w.ground_id,
    g.building_objectid,
    g.surface_objectid,
    g.geom,
    w.score_len AS score,
    'ground_wall_shared_edge_len_2d' AS scoring_method
  FROM winner_table w
  JOIN grounds g
    ON g.building_feature_id = w.building_feature_id
   AND g.surface_feature_id  = w.ground_id

  UNION ALL

  -- single-ground buildings (no scoring needed)
  SELECT
    sg.building_feature_id,
    sg.ground_id,
    sg.building_objectid,
    sg.surface_objectid,
    sg.geom,
    NULL::double precision AS score,
    'only_ground_surface' AS scoring_method
  FROM single_ground sg

  UNION ALL

  -- leftovers from multi-ground buildings that didn't get selected (keep them, but mark as fallback)
  SELECT
    umg.building_feature_id,
    umg.ground_id,
    umg.building_objectid,
    umg.surface_objectid,
    umg.geom,
    NULL::double precision AS score,
    'unresolved_no_positive_shared_len' AS scoring_method
  FROM unresolved_multi_grounds umg
),


to_insert_dedup AS (
  SELECT DISTINCT ON (ground_id)
    building_feature_id,
    ground_id,
    building_objectid,
    surface_objectid,
    geom,
    score,
    scoring_method
  FROM to_insert
  ORDER BY
    ground_id,
    -- priority: rows WITH a score first
    (score IS NULL) ASC,
    score DESC NULLS LAST,
    -- tie-breaker: prefer explicit single-ground over unresolved
    CASE scoring_method
      WHEN 'ground_wall_shared_edge_len_2d' THEN 1
      WHEN 'only_ground_surface' THEN 2
      WHEN 'unresolved_no_positive_shared_len' THEN 3
      ELSE 9
    END ASC
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
  geom
)
SELECT
  {lod_level} AS lod,
  tid.ground_id AS surface_feature_id,
  tid.building_feature_id,
  tid.building_objectid,
  tid.surface_objectid,
  710 AS objectclass_id,
  'GroundSurface' AS classname,
  tid.score,
  tid.scoring_method,
  tid.geom
FROM to_insert_dedup tid
ON CONFLICT (lod, surface_feature_id) DO UPDATE
SET building_feature_id = EXCLUDED.building_feature_id,
    objectclass_id      = EXCLUDED.objectclass_id,
    classname           = EXCLUDED.classname,
    building_objectid   = EXCLUDED.building_objectid,
    surface_objectid    = EXCLUDED.surface_objectid,
    score               = EXCLUDED.score,
    scoring_method      = EXCLUDED.scoring_method,
    geom                = EXCLUDED.geom;