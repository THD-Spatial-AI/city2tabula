-- ============================================================
-- 04_resolve_child_feat.sql (INTERNAL-ONLY: uses stage 1/2/3 tables)
-- ============================================================

-- 1) Build per-building "shell" from dumped polygons (clean POLYGONZ)
WITH building_shell AS (
  SELECT
    d.building_feature_id,
    ST_UnaryUnion(ST_Collect(d.geom))::geometry(MultiPolygonZ, {srid}) AS shell_geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature_surface d
  GROUP BY d.building_feature_id
),

-- 2) Aggregate per surface part attributes (optional but useful)
--    We’ll score by intersecting *dumped parts* against building shell.
surface_parts AS (
  SELECT
    s.building_feature_id,
    s.surface_feature_id,
    s.objectclass_id,
    s.classname,
    s.child_row_id,
    s.geom,
    s.surface_area,
    s.tilt,
    s.azimuth,
    s.is_planar,
    s.is_valid
  FROM {city2tabula_schema}.{lod_schema}_child_feature_surface s
  WHERE s.building_feature_id IS NOT NULL
    AND s.surface_feature_id IS NOT NULL
),

-- 3) Candidate pairs (from stage 1)
cand AS (
  SELECT
    c.lod,
    c.building_feature_id,
    c.surface_feature_id,
    c.objectclass_id,
    c.classname
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  WHERE c.lod = {lod_level}
    AND c.building_feature_id IN {building_ids}
),

-- 4) Score each candidate by summing overlap of dumped parts with the building shell
--    NOTE: use ONLY planar parts to avoid SFCGAL blowing up on garbage
scored AS (
  SELECT
    c.lod,
    c.building_feature_id,
    c.surface_feature_id,
    MIN(sp.objectclass_id) AS objectclass_id,
    MIN(sp.classname) AS classname,

    -- total area of parts for this (building,surface) "view"
    SUM(CASE WHEN sp.is_planar THEN COALESCE(sp.surface_area, 0) ELSE 0 END) AS surf_area_sum,

    -- overlap area: parts intersected with this building's shell
    SUM(
      CASE
        WHEN sp.is_planar
         AND ST_3DIntersects(sp.geom, bs.shell_geom)
         AND ST_Dimension(ST_CollectionExtract(ST_3DIntersection(sp.geom, bs.shell_geom), 3)) = 2
        THEN
          CG_3DArea(ST_CollectionExtract(ST_3DIntersection(sp.geom, bs.shell_geom), 3))
        ELSE 0
      END
    ) AS inter_area_sum,

    AVG(sp.tilt) AS tilt_avg,
    AVG(sp.azimuth) AS azimuth_avg

  FROM cand c
  JOIN building_shell bs
    ON bs.building_feature_id = c.building_feature_id
  JOIN surface_parts sp
    ON sp.building_feature_id = c.building_feature_id
   AND sp.surface_feature_id = c.surface_feature_id

  GROUP BY c.lod, c.building_feature_id, c.surface_feature_id
),

final_score AS (
  SELECT
    *,
    (inter_area_sum / NULLIF(surf_area_sum, 0)) AS score,
    CASE
      WHEN classname = 'WallSurface' THEN 'wall_dump_3d_ratio'
      WHEN classname IN ('RoofSurface','GroundSurface') THEN 'top_dump_3d_ratio'
      ELSE 'dump_3d_ratio'
    END AS scoring_method
  FROM scored
),

ranked AS (
  SELECT
    *,
    ROW_NUMBER() OVER (PARTITION BY surface_feature_id ORDER BY score DESC) AS rn,
    LEAD(building_feature_id) OVER (PARTITION BY surface_feature_id ORDER BY score DESC) AS second_building_id,
    LEAD(score) OVER (PARTITION BY surface_feature_id ORDER BY score DESC) AS second_score
  FROM final_score
),

picked AS (
  SELECT *
  FROM ranked
  WHERE rn = 1
    AND score >= 0.70
),

-- 5) Insert resolved ownership using geom from stage 1 candidate table
ins_resolved AS (
  INSERT INTO {city2tabula_schema}.{lod_schema}_child_feature_resolved (
    id, lod,
    surface_feature_id, building_feature_id,
    objectclass_id, classname,
    score, scoring_method,
    is_party_wall,
    geom
  )
  SELECT
    gen_random_uuid(),
    {lod_level},
    p.surface_feature_id,
    p.building_feature_id,
    p.objectclass_id,
    p.classname,
    p.score,
    p.scoring_method,

    (p.classname = 'WallSurface'
      AND p.score >= 0.70
      AND COALESCE(p.second_score,0) >= 0.30
      AND COALESCE(p.second_building_id,-1) <> p.building_feature_id
    ) AS is_party_wall,

    c.geom
  FROM picked p
  JOIN {city2tabula_schema}.{lod_schema}_child_feature c
    ON c.lod = p.lod
   AND c.building_feature_id = p.building_feature_id
   AND c.surface_feature_id = p.surface_feature_id

  ON CONFLICT (lod, surface_feature_id) DO UPDATE
    SET building_feature_id = EXCLUDED.building_feature_id,
        objectclass_id      = EXCLUDED.objectclass_id,
        classname           = EXCLUDED.classname,
        score               = EXCLUDED.score,
        scoring_method      = EXCLUDED.scoring_method,
        is_party_wall       = EXCLUDED.is_party_wall,
        geom                = EXCLUDED.geom
  RETURNING *
),

-- 6) Insert party wall links (A<->B) with shared area computed from dumped parts
party_pairs AS (
  SELECT
    r.lod,
    LEAST(r.building_feature_id, p.second_building_id) AS building_a_id,
    GREATEST(r.building_feature_id, p.second_building_id) AS building_b_id,
    r.surface_feature_id AS surface_a_id,
    NULL::bigint AS surface_b_id
  FROM ins_resolved r
  JOIN picked p
    ON p.surface_feature_id = r.surface_feature_id
  WHERE r.is_party_wall = TRUE
),

party_area AS (
  SELECT
    pp.lod,
    pp.building_a_id,
    pp.building_b_id,
    pp.surface_a_id,
    pp.surface_b_id,

    -- shared area as overlap of the winning surface parts with the other building shell
    SUM(
      CASE
        WHEN sp.is_planar
         AND ST_3DIntersects(sp.geom, bs.shell_geom)
         AND ST_Dimension(ST_CollectionExtract(ST_3DIntersection(sp.geom, bs.shell_geom), 3)) = 2
        THEN CG_3DArea(ST_CollectionExtract(ST_3DIntersection(sp.geom, bs.shell_geom), 3))
        ELSE 0
      END
    ) AS shared_area

  FROM party_pairs pp
  JOIN building_shell bs
    ON bs.building_feature_id = pp.building_b_id
  JOIN surface_parts sp
    ON sp.building_feature_id = pp.building_a_id
   AND sp.surface_feature_id = pp.surface_a_id

  GROUP BY pp.lod, pp.building_a_id, pp.building_b_id, pp.surface_a_id, pp.surface_b_id
)

INSERT INTO {city2tabula_schema}.{lod_schema}_party_wall (
  id, lod,
  building_a_id, building_b_id,
  surface_a_id, surface_b_id,
  shared_area, shared_area_unit,
  shared_geom
)
SELECT
  gen_random_uuid(),
  {lod_level},
  building_a_id,
  building_b_id,
  surface_a_id,
  surface_b_id,
  shared_area,
  'sqm',
  NULL::geometry
FROM party_area
WHERE shared_area IS NOT NULL AND shared_area > 0
ON CONFLICT (lod, building_a_id, building_b_id, surface_a_id, surface_b_id) DO NOTHING;
