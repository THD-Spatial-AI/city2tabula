-- Computes building volume as a sum of per-solid bounding-block approximations:
-- height × footprint_area of each row in _building_part. min_volume uses eave height
-- (wall-only span); max_volume uses ridge height (wall + roof span). A building of
-- one solid gets exactly min_height × footprint_area.
WITH part_volumes AS (
    SELECT
        building_feature_id,
        SUM(min_height * footprint_area) AS min_volume,
        SUM(max_height * footprint_area) AS max_volume
    FROM {city2tabula_schema}.{lod_schema}_building_part
    WHERE building_feature_id IN {building_ids}
    GROUP BY building_feature_id
)
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET
    -- Volume calculations
    min_volume = CASE
        WHEN pv.min_volume IS NOT NULL
        THEN ROUND(pv.min_volume::numeric, 2)
        ELSE bf.min_volume
    END,
    max_volume = CASE
        WHEN pv.max_volume IS NOT NULL
        THEN ROUND(pv.max_volume::numeric, 2)
        ELSE bf.max_volume
    END,
    min_volume_unit = CASE
        WHEN pv.min_volume IS NOT NULL
        THEN 'cbm'
        ELSE bf.min_volume_unit
    END,
    max_volume_unit = CASE
        WHEN pv.max_volume IS NOT NULL
        THEN 'cbm'
        ELSE bf.max_volume_unit
    END
FROM part_volumes pv
WHERE bf.building_feature_id = pv.building_feature_id;
