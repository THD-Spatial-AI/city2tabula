-- area_total_wall and surface_count_wall from the exterior wall pieces, area_party_wall from the
-- shared ones. Runs before 05_label_buildings.sql, so matching sees envelope walls the way TABULA
-- counts them. Only rows whose values change are written.

WITH totals AS (
    SELECT b.id,
           ROUND(COALESCE(SUM(s.surface_area) FILTER (
               WHERE s.surface_type = 'WallSurface' AND NOT s.is_party_wall), 0)::numeric, 2) AS envelope,
           ROUND(COALESCE(SUM(s.surface_area) FILTER (
               WHERE s.surface_type = 'WallSurface' AND s.is_party_wall), 0)::numeric, 2) AS party,
           COUNT(*) FILTER (WHERE s.surface_type = 'WallSurface' AND NOT s.is_party_wall) AS envelope_count
    FROM {city2tabula_schema}.{lod_schema}_building b
    LEFT JOIN {city2tabula_schema}.{lod_schema}_surface s ON s.building_object_id = b.object_id
    GROUP BY b.id
)
UPDATE {city2tabula_schema}.{lod_schema}_building b
SET area_total_wall = t.envelope,
    area_party_wall = t.party,
    area_party_wall_unit = 'sqm',
    surface_count_wall = t.envelope_count
FROM totals t
WHERE b.id = t.id
  AND (b.area_total_wall, b.area_party_wall, b.surface_count_wall)
      IS DISTINCT FROM (t.envelope, t.party, t.envelope_count);
