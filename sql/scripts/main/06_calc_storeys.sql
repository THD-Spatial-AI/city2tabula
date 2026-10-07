-- Storey counts and heated floor area. Sources and validation:
-- docs/code/sql-pipeline/06-storeys.md.
--
--   full_storeys      = max(1, round(min_height / storey_height)): storeys below the
--                       eave, rounded to the nearest storey. TABULA's n_Storey counts the
--                       same: complete storeys without attic or cellar.
--   attic_storey      = the roof space reaches 2 m clear height, the height from which
--                       WoFlV § 4 counts floor area in full; taken as ridge - eave >= 2 m.
--   number_of_storeys = full_storeys + attic_storey.
--   area_total_floor  = footprint_area × full_storeys, plus attic_floor_area (script 04)
--                       when the attic counts as a storey.
WITH storeys AS (
    SELECT
        id,
        CASE WHEN min_height > 0 AND storey_height > 0
             THEN GREATEST(1, ROUND(min_height / storey_height))::integer
             ELSE 1
        END AS full_storeys,
        COALESCE(max_height - min_height >= 2.0, FALSE) AS attic_storey
    FROM {city2tabula_schema}.{lod_schema}_building
    WHERE building_feature_id IN {building_ids}
)
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET
    full_storeys = s.full_storeys,
    attic_storey = s.attic_storey,
    number_of_storeys = s.full_storeys + s.attic_storey::integer,
    area_total_floor = ROUND((bf.footprint_area * s.full_storeys
        + CASE WHEN s.attic_storey THEN COALESCE(bf.attic_floor_area, 0) ELSE 0 END)::numeric, 2),
    area_total_floor_unit = 'sqm'
FROM storeys s
WHERE bf.id = s.id;
