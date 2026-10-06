-- Assigns each building its best-matching TABULA variant code using nearest-neighbour
-- matching over 9 dimensions (volume, footprint, storeys, footprint and roof
-- complexity, attached neighbours, roof, wall and floor area). Runs once after every
-- batch, following neighbour detection (01_detect_neighbours.sql), because the
-- normalisation spans all buildings.
--
-- Each dimension is min-max normalised over buildings and variants together, so both
-- sides share one scale (minmax_norm). TABULA leaves many variant values empty; the
-- extraction stores them as NULL. The distance is the root mean square over the
-- dimensions both sides know, as in Gower's (1971) coefficient: a plain sum would
-- favour variants with fewer known values. A dimension with no spread drops out too.
--
-- Gower, J. C. (1971). A general coefficient of similarity and some of its
-- properties. Biometrics 27(4), 857-871. https://doi.org/10.2307/2528823

WITH buildings AS (
  -- full_storeys, not number_of_storeys: TABULA's n_Storey counts complete storeys
  -- without the attic (script 06).
  SELECT building_feature_id, max_volume, footprint_area, full_storeys AS number_of_storeys,
         footprint_complexity, roof_complexity, attached_neighbour_class,
         area_total_roof, area_total_wall, area_total_floor
  FROM {city2tabula_schema}.{lod_schema}_building
  WHERE footprint_area IS NOT NULL
    AND full_storeys IS NOT NULL
    AND area_total_roof IS NOT NULL
    AND area_total_wall IS NOT NULL
    AND area_total_floor IS NOT NULL
),
stats AS (
  SELECT
    MIN(max_volume)               AS lo_vol,     MAX(max_volume)               AS hi_vol,
    MIN(footprint_area)           AS lo_area,    MAX(footprint_area)           AS hi_area,
    MIN(number_of_storeys)        AS lo_storeys, MAX(number_of_storeys)        AS hi_storeys,
    MIN(footprint_complexity)     AS lo_fc,      MAX(footprint_complexity)     AS hi_fc,
    MIN(roof_complexity)          AS lo_rc,      MAX(roof_complexity)          AS hi_rc,
    MIN(attached_neighbour_class) AS lo_ac,      MAX(attached_neighbour_class) AS hi_ac,
    MIN(area_total_roof)          AS lo_roof,    MAX(area_total_roof)          AS hi_roof,
    MIN(area_total_wall)          AS lo_wall,    MAX(area_total_wall)          AS hi_wall,
    MIN(area_total_floor)         AS lo_floor,   MAX(area_total_floor)         AS hi_floor
  FROM (
    SELECT max_volume, footprint_area, number_of_storeys, footprint_complexity,
           roof_complexity, attached_neighbour_class, area_total_roof, area_total_wall,
           area_total_floor
    FROM buildings
    UNION ALL
    SELECT max_volume, footprint_area, number_of_storeys, footprint_complexity,
           roof_complexity, attached_neighbour_class, area_total_roof, area_total_wall,
           area_total_floor
    FROM {city2tabula_schema}.tabula_variant
  ) all_data
),
ranked AS (
  SELECT b.building_feature_id,
         v.tabula_variant_code_id,
         v.tabula_variant_code,
         ROW_NUMBER() OVER (PARTITION BY b.building_feature_id
                            ORDER BY d.distance, v.tabula_variant_code_id) AS rnk
  FROM buildings b
  CROSS JOIN {city2tabula_schema}.tabula_variant v
  CROSS JOIN stats s
  CROSS JOIN LATERAL (
    SELECT sqrt(avg(term)) AS distance
    FROM (VALUES
      (power({city2tabula_schema}.minmax_norm(b.max_volume, s.lo_vol, s.hi_vol)
           - {city2tabula_schema}.minmax_norm(v.max_volume, s.lo_vol, s.hi_vol), 2)),
      (power({city2tabula_schema}.minmax_norm(b.footprint_area, s.lo_area, s.hi_area)
           - {city2tabula_schema}.minmax_norm(v.footprint_area, s.lo_area, s.hi_area), 2)),
      (power({city2tabula_schema}.minmax_norm(b.number_of_storeys, s.lo_storeys, s.hi_storeys)
           - {city2tabula_schema}.minmax_norm(v.number_of_storeys, s.lo_storeys, s.hi_storeys), 2)),
      (power({city2tabula_schema}.minmax_norm(b.footprint_complexity, s.lo_fc, s.hi_fc)
           - {city2tabula_schema}.minmax_norm(v.footprint_complexity, s.lo_fc, s.hi_fc), 2)),
      (power({city2tabula_schema}.minmax_norm(b.roof_complexity, s.lo_rc, s.hi_rc)
           - {city2tabula_schema}.minmax_norm(v.roof_complexity, s.lo_rc, s.hi_rc), 2)),
      (power({city2tabula_schema}.minmax_norm(b.attached_neighbour_class, s.lo_ac, s.hi_ac)
           - {city2tabula_schema}.minmax_norm(v.attached_neighbour_class, s.lo_ac, s.hi_ac), 2)),
      (power({city2tabula_schema}.minmax_norm(b.area_total_roof, s.lo_roof, s.hi_roof)
           - {city2tabula_schema}.minmax_norm(v.area_total_roof, s.lo_roof, s.hi_roof), 2)),
      (power({city2tabula_schema}.minmax_norm(b.area_total_wall, s.lo_wall, s.hi_wall)
           - {city2tabula_schema}.minmax_norm(v.area_total_wall, s.lo_wall, s.hi_wall), 2)),
      (power({city2tabula_schema}.minmax_norm(b.area_total_floor, s.lo_floor, s.hi_floor)
           - {city2tabula_schema}.minmax_norm(v.area_total_floor, s.lo_floor, s.hi_floor), 2))
    ) terms(term)
  ) d
  WHERE d.distance IS NOT NULL
)
UPDATE {city2tabula_schema}.{lod_schema}_building bf
SET tabula_variant_code_id = ranked.tabula_variant_code_id,
    tabula_variant_code    = ranked.tabula_variant_code
FROM ranked
WHERE bf.building_feature_id = ranked.building_feature_id
  AND ranked.rnk = 1;
