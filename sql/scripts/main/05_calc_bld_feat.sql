WITH batch_buildings AS (
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved
  WHERE lod = {lod_level}
    AND building_feature_id IN {building_ids}
),

-- resolved ground geometry per building (should be one row per building after your resolver)
resolved_ground AS (
  SELECT
    r.building_feature_id,
    r.geom AS ground_geom_mpz
  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved r
  WHERE r.lod = {lod_level}
    AND r.classname = 'GroundSurface'
    AND r.building_feature_id IN {building_ids}
),

-- aggregate surface attributes per building, but only for resolved surfaces
resolved_attr AS (
  SELECT
    r.building_feature_id,

    SUM(s.surface_area) FILTER (WHERE r.classname = 'GroundSurface') AS footprint_area,
    SUM(s.surface_area) FILTER (WHERE r.classname = 'RoofSurface')   AS area_total_roof,
    SUM(s.surface_area) FILTER (WHERE r.classname = 'WallSurface')   AS area_total_wall,
    SUM(s.surface_area) FILTER (WHERE r.classname = 'GroundSurface') AS area_total_floor,

    COUNT(*) FILTER (WHERE r.classname = 'RoofSurface')   AS surface_count_roof,
    COUNT(*) FILTER (WHERE r.classname = 'WallSurface')   AS surface_count_wall,
    COUNT(*) FILTER (WHERE r.classname = 'GroundSurface') AS surface_count_floor,

    -- pick wall height stats from resolved wall parts (Stage 3 stores per polygon)
    MIN(s.height) FILTER (WHERE r.classname = 'WallSurface') AS min_height,
    MAX(s.height) FILTER (WHERE r.classname = 'WallSurface') AS max_height_wall

  FROM {city2tabula_schema}.{lod_schema}_child_feature_resolved r
  JOIN {city2tabula_schema}.{lod_schema}_child_feature_surface s
    ON s.building_feature_id = r.building_feature_id
   AND s.surface_feature_id  = r.surface_feature_id
  WHERE r.lod = {lod_level}
    AND r.building_feature_id IN {building_ids}
    AND s.is_planar = true
  GROUP BY r.building_feature_id
),

-- build footprint geometry + centroid from resolved ground geometry
footprint_geom AS (
  SELECT
    g.building_feature_id,

    -- footprint as 2D multipolygon (keep in target SRID)
    ST_Multi(ST_Buffer(ST_Force2D(g.ground_geom_mpz), 0))::geometry(MultiPolygon, {srid}) AS footprint_2d,

    ST_Centroid(
      ST_Multi(ST_Buffer(ST_Force2D(g.ground_geom_mpz), 0))
    )::geometry(Point, {srid}) AS centroid_2d,

    ST_NPoints(
      ST_Boundary(ST_Multi(ST_Buffer(ST_Force2D(g.ground_geom_mpz), 0)))
    ) AS footprint_boundary_npoints

  FROM resolved_ground g
),

building_data AS (
  SELECT
    b.building_feature_id,

    COALESCE(a.footprint_area, 0) AS footprint_area,

    CASE
      WHEN f.footprint_boundary_npoints <= 4 THEN 0
      WHEN f.footprint_boundary_npoints BETWEEN 5 AND 10 THEN 1
      ELSE 2
    END AS footprint_complexity,

    CASE
      WHEN COALESCE(a.surface_count_roof, 0) = 1 THEN 0
      WHEN COALESCE(a.surface_count_roof, 0) BETWEEN 2 AND 4 THEN 1
      ELSE 2
    END AS roof_complexity,

    FALSE AS has_attached_neighbour,
    ARRAY[]::BIGINT[] AS attached_neighbour_id,
    0 AS total_attached_neighbour,

    COALESCE(a.area_total_roof, 0) AS area_total_roof,
    'sqm' AS area_total_roof_unit,

    COALESCE(a.area_total_wall, 0) AS area_total_wall,
    'sqm' AS area_total_wall_unit,

    COALESCE(a.area_total_floor, 0) AS area_total_floor,
    'sqm' AS area_total_floor_unit,

    COALESCE(a.surface_count_roof, 0) AS surface_count_roof,
    COALESCE(a.surface_count_wall, 0) AS surface_count_wall,
    COALESCE(a.surface_count_floor, 0) AS surface_count_floor,

    COALESCE(a.min_height, 0) AS min_height,
    'm' AS min_height_unit,

    -- wall height + roof height is dataset-dependent; keep simple
    COALESCE(a.max_height_wall, 0) AS max_height,
    'm' AS max_height_unit,

    2.5 AS room_height,
    'm' AS room_height_unit,

    CASE
      WHEN COALESCE(a.max_height_wall, 0) > 0 THEN GREATEST(1, FLOOR(a.max_height_wall / 2.5))::int
      ELSE 1
    END AS number_of_storeys,

    f.centroid_2d AS building_centroid_geom,
    f.footprint_2d::geometry(MultiPolygon, {srid}) AS building_footprint_geom

  FROM batch_buildings b
  LEFT JOIN resolved_attr a ON a.building_feature_id = b.building_feature_id
  LEFT JOIN footprint_geom f ON f.building_feature_id = b.building_feature_id
)

INSERT INTO {city2tabula_schema}.{lod_schema}_building_feature (
  id,
  building_feature_id,
  footprint_area,
  footprint_complexity,
  roof_complexity,
  has_attached_neighbour,
  attached_neighbour_id,
  total_attached_neighbour,
  area_total_roof,
  area_total_roof_unit,
  area_total_wall,
  area_total_wall_unit,
  area_total_floor,
  area_total_floor_unit,
  surface_count_floor,
  surface_count_roof,
  surface_count_wall,
  min_height,
  min_height_unit,
  max_height,
  max_height_unit,
  room_height,
  room_height_unit,
  number_of_storeys,
  building_centroid_geom,
  building_footprint_geom
)
SELECT
  gen_random_uuid(),
  building_feature_id,
  footprint_area,
  footprint_complexity,
  roof_complexity,
  has_attached_neighbour,
  attached_neighbour_id,
  total_attached_neighbour,
  area_total_roof,
  area_total_roof_unit,
  area_total_wall,
  area_total_wall_unit,
  area_total_floor,
  area_total_floor_unit,
  surface_count_floor,
  surface_count_roof,
  surface_count_wall,
  min_height,
  min_height_unit,
  max_height,
  max_height_unit,
  room_height,
  room_height_unit,
  number_of_storeys,
  building_centroid_geom,
  building_footprint_geom
FROM building_data
ON CONFLICT (building_feature_id) DO UPDATE
SET footprint_area          = EXCLUDED.footprint_area,
    footprint_complexity    = EXCLUDED.footprint_complexity,
    roof_complexity         = EXCLUDED.roof_complexity,
    area_total_roof         = EXCLUDED.area_total_roof,
    area_total_wall         = EXCLUDED.area_total_wall,
    area_total_floor        = EXCLUDED.area_total_floor,
    surface_count_floor     = EXCLUDED.surface_count_floor,
    surface_count_roof      = EXCLUDED.surface_count_roof,
    surface_count_wall      = EXCLUDED.surface_count_wall,
    min_height              = EXCLUDED.min_height,
    max_height              = EXCLUDED.max_height,
    number_of_storeys       = EXCLUDED.number_of_storeys,
    building_centroid_geom  = EXCLUDED.building_centroid_geom,
    building_footprint_geom = EXCLUDED.building_footprint_geom;