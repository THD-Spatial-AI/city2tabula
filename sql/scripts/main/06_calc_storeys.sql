-- Refines number_of_storeys as min_height / room_height (both in metres); min_height
-- is the tallest solid's eave height (script 04).
-- Also overwrites area_total_floor with the total heated floor area estimate: each
-- solid's footprint_area × its own storey count (eave height / room_height, falling
-- back to 1), summed over the building's rows in _building_part. A building of one
-- solid gets exactly footprint_area × number_of_storeys.
WITH part_floors AS (
    SELECT
        p.building_feature_id,
        SUM(p.footprint_area * CASE
            WHEN bf.room_height > 0 AND p.min_height > 0
            THEN (p.min_height / bf.room_height)::integer
            ELSE 1
        END) AS floor_area
    FROM {city2tabula_schema}.{lod_schema}_building_part p
    JOIN {city2tabula_schema}.{lod_schema}_building bf ON bf.building_feature_id = p.building_feature_id
    WHERE p.building_feature_id IN {building_ids}
    GROUP BY p.building_feature_id
)
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET
    -- Number of storeys calculation
    number_of_storeys = CASE
        WHEN bf.room_height IS NOT NULL AND bf.min_height IS NOT NULL
             AND bf.room_height > 0 AND bf.min_height > 0
        THEN bf.min_height / bf.room_height
        ELSE bf.number_of_storeys
    END,
    room_height_unit = CASE
        WHEN bf.room_height IS NOT NULL
        THEN 'm'
        ELSE bf.room_height_unit
    END,
    area_total_floor = ROUND(pf.floor_area::numeric, 2),
    area_total_floor_unit = 'sqm'
FROM part_floors pf
WHERE bf.building_feature_id = pf.building_feature_id;
