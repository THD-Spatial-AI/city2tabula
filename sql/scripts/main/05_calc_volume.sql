-- Computes building volume as a bounding-box approximation: height × footprint_area.
-- min_volume uses eave height (wall-only span); max_volume uses ridge height (wall + roof span).
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET
    -- Volume calculations
    min_volume = CASE
        WHEN bf.min_height IS NOT NULL AND bf.footprint_area IS NOT NULL
        THEN bf.min_height * bf.footprint_area
        ELSE bf.min_volume
    END,
    max_volume = CASE
        WHEN bf.max_height IS NOT NULL AND bf.footprint_area IS NOT NULL
        THEN bf.max_height * bf.footprint_area
        ELSE bf.max_volume
    END,
    min_volume_unit = CASE
        WHEN bf.min_height IS NOT NULL AND bf.footprint_area IS NOT NULL
        THEN 'cbm'
        ELSE bf.min_volume_unit
    END,
    max_volume_unit = CASE
        WHEN bf.max_height IS NOT NULL AND bf.footprint_area IS NOT NULL
        THEN 'cbm'
        ELSE bf.max_volume_unit
    END
WHERE bf.building_feature_id IN {building_ids};