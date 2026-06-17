-- Refines number_of_storeys as min_height / room_height (both in metres).
-- Also overwrites area_total_floor with footprint_area × number_of_storeys, replacing
-- the raw GroundSurface area sum from script 04 with a total heated floor area estimate.
-- Note: area_total_floor in the SET clause uses the OLD number_of_storeys value (pre-update)
-- because PostgreSQL evaluates all SET expressions from the row state before the UPDATE.
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
    area_total_floor = bf.footprint_area * bf.number_of_storeys,
    area_total_floor_unit = 'sqm'
WHERE bf.building_feature_id IN {building_ids};