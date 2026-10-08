INSERT INTO {city2tabula_schema}.{tabula_variant_table} (
    tabula_variant_code_id,
    tabula_variant_code,
    construction_year_1,
    construction_year_2,
    max_volume,
    total_area,
    footprint_area,
    number_of_storeys,
    footprint_complexity,
    attached_neighbour_class,
    roof_complexity,
    area_total_roof,
    area_total_wall,
    area_total_floor,
    building_size_class
)
-- TABULA marks a missing value with 0 or leaves it empty, and every matching input here
-- becomes NULL in that case so matching skips it (post/05_label_buildings.sql). The
-- footprint is A_C_ExtDim per storey, the external-dimension area comparable with a
-- LoD2 footprint; A_C_National is each country's own reference area.
SELECT
    id AS tabula_variant_code_id,
    "Code_BuildingVariant" AS tabula_variant_code,
    "Year1_Building" AS construction_year_1,
    "Year2_Building" AS construction_year_2,
    NULLIF("V_C", 0) AS max_volume,
    NULLIF("A_C_National", 0) AS total_area,
    NULLIF("A_C_ExtDim", 0) / NULLIF("n_Storey", 0) AS footprint_area,
    NULLIF("n_Storey", 0) AS number_of_storeys,
    CASE "Code_ComplexFootprint"
        WHEN 'Simple' THEN 0
        WHEN 'Regular' THEN 1
        WHEN 'Complex' THEN 2
    END AS footprint_complexity,
    CASE "Code_AttachedNeighbours"
        WHEN 'B_Alone' THEN 0
        WHEN 'B_N1' THEN 1
        WHEN 'B_N2' THEN 2
    END AS attached_neighbour_class,
    CASE "Code_ComplexRoof"
        WHEN 'Simple' THEN 0
        WHEN 'Regular' THEN 1
        WHEN 'Complex' THEN 2
    END AS roof_complexity,
    NULLIF(COALESCE("A_Roof_1", 0) + COALESCE("A_Roof_2", 0), 0) AS area_total_roof,
    NULLIF(COALESCE("A_Wall_1", 0) + COALESCE("A_Wall_2", 0) + COALESCE("A_Wall_3", 0), 0) AS area_total_wall,
    NULLIF("A_C_ExtDim", 0) AS area_total_floor,
    CASE "Code_BuildingSizeClass"
        WHEN 'SFH' THEN 0 -- Single Family House
        WHEN 'MFH' THEN 1 -- Multi Family House
        WHEN 'TH' THEN 2 -- Terraced House
        WHEN 'AB' THEN 3 -- Apartment Building
        ELSE -1 -- Unknown or not applicable
    END AS building_size_class
FROM
    {tabula_schema}.{tabula_table}
WHERE
    "Code_BuildingVariant" IS NOT NULL
    AND "Number_BuildingVariant" = 1 -- Only insert the first variant for each building
ON CONFLICT (tabula_variant_code_id) DO NOTHING; -- tabula_variant is country-wide static data,
    -- re-imported unchanged on every incremental -import-data/on-request run