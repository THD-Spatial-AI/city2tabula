-- Label LoD2 building features with TABULA variant codes
-- This script finds the best matching TABULA variant for each building based on normalized feature distances

WITH stats AS (
  SELECT
    MIN(max_volume) AS min_vol, MAX(max_volume) AS max_vol,
    MIN(footprint_area) AS min_area, MAX(footprint_area) AS max_area,
    MIN(number_of_storeys) AS min_storeys, MAX(number_of_storeys) AS max_storeys,
    MIN(attached_neighbour_class) AS min_neigh, MAX(attached_neighbour_class) AS max_neigh,
    MIN(footprint_complexity) AS min_fc, MAX(footprint_complexity) AS max_fc,
    MIN(roof_complexity) AS min_rc, MAX(roof_complexity) AS max_rc,
    MIN(area_total_roof) AS min_roof, MAX(area_total_roof) AS max_roof,
    MIN(area_total_wall) AS min_wall, MAX(area_total_wall) AS max_wall,
    MIN(area_total_floor) AS min_floor, MAX(area_total_floor) AS max_floor
  FROM (
    SELECT max_volume, footprint_area, number_of_storeys, attached_neighbour_class,
           footprint_complexity, roof_complexity, area_total_roof, area_total_wall, area_total_floor
    FROM {city2tabula_schema}.{lod_schema}_building_feature
    WHERE footprint_area IS NOT NULL
      AND number_of_storeys IS NOT NULL
      AND area_total_roof IS NOT NULL
      AND area_total_wall IS NOT NULL
      AND area_total_floor IS NOT NULL
    UNION ALL
    SELECT max_volume, footprint_area, number_of_storeys, attached_neighbour_class,
           footprint_complexity, roof_complexity, area_total_roof, area_total_wall, area_total_floor
    FROM {city2tabula_schema}.tabula_variant
    WHERE max_volume IS NOT NULL
      AND footprint_area IS NOT NULL
      AND number_of_storeys IS NOT NULL
      AND area_total_roof IS NOT NULL
      AND area_total_wall IS NOT NULL
      AND area_total_floor IS NOT NULL
  ) all_data
),
ranked AS (
  SELECT b.building_feature_id,
         v.tabula_variant_code_id,
         v.tabula_variant_code,
         ROW_NUMBER() OVER (
           PARTITION BY b.building_feature_id
           ORDER BY sqrt(
             power(COALESCE(((b.max_volume - s.min_vol) / NULLIF(s.max_vol-s.min_vol,0)), 0) -
                   COALESCE(((v.max_volume - s.min_vol) / NULLIF(s.max_vol-s.min_vol,0)), 0), 2) +
             power(COALESCE(((b.footprint_area - s.min_area) / NULLIF(s.max_area-s.min_area,0)), 0) -
                   COALESCE(((v.footprint_area - s.min_area) / NULLIF(s.max_area-s.min_area,0)), 0), 2) +
             power(COALESCE(((b.number_of_storeys - s.min_storeys) / NULLIF(s.max_storeys-s.min_storeys,0)), 0) -
                   COALESCE(((v.number_of_storeys - s.min_storeys) / NULLIF(s.max_storeys-s.min_storeys,0)), 0), 2) +
             power(COALESCE(((b.attached_neighbour_class - s.min_neigh) / NULLIF(s.max_neigh-s.min_neigh,0)), 0) -
                   COALESCE(((v.attached_neighbour_class - s.min_neigh) / NULLIF(s.max_neigh-s.min_neigh,0)), 0), 2) +
             power(COALESCE(((b.footprint_complexity - s.min_fc) / NULLIF(s.max_fc-s.min_fc,0)), 0) -
                   COALESCE(((v.footprint_complexity - s.min_fc) / NULLIF(s.max_fc-s.min_fc,0)), 0), 2) +
             power(COALESCE(((b.roof_complexity - s.min_rc) / NULLIF(s.max_rc-s.min_rc,0)), 0) -
                   COALESCE(((v.roof_complexity - s.min_rc) / NULLIF(s.max_rc-s.min_rc,0)), 0), 2) +
             power(COALESCE(((b.area_total_roof - s.min_roof) / NULLIF(s.max_roof-s.min_roof,0)), 0) -
                   COALESCE(((v.area_total_roof - s.min_roof) / NULLIF(s.max_roof-s.min_roof,0)), 0), 2) +
             power(COALESCE(((b.area_total_wall - s.min_wall) / NULLIF(s.max_wall-s.min_wall,0)), 0) -
                   COALESCE(((v.area_total_wall - s.min_wall) / NULLIF(s.max_wall-s.min_wall,0)), 0), 2) +
             power(COALESCE(((b.area_total_floor - s.min_floor) / NULLIF(s.max_floor-s.min_floor,0)), 0) -
                   COALESCE(((v.area_total_floor - s.min_floor) / NULLIF(s.max_floor-s.min_floor,0)), 0), 2)
           ) ASC
         ) AS rnk
  FROM {city2tabula_schema}.{lod_schema}_building_feature b
  CROSS JOIN {city2tabula_schema}.tabula_variant v
  CROSS JOIN stats s
  WHERE b.footprint_area IS NOT NULL
    AND b.number_of_storeys IS NOT NULL
    AND b.area_total_roof IS NOT NULL
    AND b.area_total_wall IS NOT NULL
    AND b.area_total_floor IS NOT NULL
    AND v.max_volume IS NOT NULL
    AND v.footprint_area IS NOT NULL
    AND v.number_of_storeys IS NOT NULL
    AND v.area_total_roof IS NOT NULL
    AND v.area_total_wall IS NOT NULL
    AND v.area_total_floor IS NOT NULL
)
UPDATE {city2tabula_schema}.{lod_schema}_building_feature bf
SET tabula_variant_code_id = ranked.tabula_variant_code_id,
    tabula_variant_code    = ranked.tabula_variant_code
FROM ranked
WHERE bf.building_feature_id = ranked.building_feature_id
  AND ranked.rnk = 1;
