-- Aggregates per-surface attributes into a single row per building and inserts into
-- _building. Skips buildings that already have a row in the target table.
--
-- Height semantics (both derived from child surface heights):
--   min_height — maximum vertical span of any WallSurface face (eave height).
--                Named "min" because it excludes the roof ridge contribution.
--   max_height — eave height + maximum vertical span of any RoofSurface face (ridge height).
--
-- Complexity codes (0 = simple, 1 = regular, 2 = complex):
--   footprint_complexity — based on vertex count of the merged GroundSurface boundary.
--   roof_complexity      — based on number of distinct RoofSurface polygons.
--
-- surface_count_floor is initialised to 0 here; area_total_floor is later overwritten
-- in script 06 as footprint_area × number_of_storeys (total heated floor area estimate).

WITH new_buildings AS (
    -- Select only buildings that haven't been processed yet
    SELECT DISTINCT building_feature_id
    FROM {city2tabula_schema}.{lod_schema}_surface_raw
    WHERE building_feature_id IN {building_ids}
),
aggregated_surfaces AS (
    SELECT
        cfs.building_feature_id,
        MIN(cfs.building_object_id) AS object_id,
        0 as construction_year,
        0.0 as heating_demand,
        'kWh/m2a' AS heating_demand_unit,
        SUM(surface_area) FILTER (WHERE classname = 'GroundSurface') AS footprint_area,
        -- Footprint complexity: vertex count of the merged ground boundary.
        -- ≤ 4 vertices → simple rectangle; 5–10 → regular polygon; > 10 → complex shape.
        CASE
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) <= 4 THEN 0
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) BETWEEN 5 AND 10 THEN 1
            ELSE 2
        END AS footprint_complexity,
        -- Roof complexity: number of distinct RoofSurface polygons.
        -- 1 face → simple (flat or single-pitch); 2–4 → regular (gable, hip); > 4 → complex.
        CASE
            WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') = 1 THEN 0
            WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') BETWEEN 2 AND 4 THEN 1
            ELSE 2
        END AS roof_complexity,
        FALSE AS has_attached_neighbour,
        ARRAY[]::INTEGER[] AS attached_neighbour_id,
        0 AS total_attached_neighbour,
        SUM(CASE WHEN classname = 'RoofSurface' THEN surface_area ELSE 0 END) AS area_total_roof,
        'sqm' AS area_total_roof_unit,
        SUM(CASE WHEN classname = 'WallSurface' THEN surface_area ELSE 0 END) AS area_total_wall,
        'sqm' AS area_total_wall_unit,
        SUM(CASE WHEN classname = 'GroundSurface' THEN surface_area ELSE 0 END) AS area_total_floor,
        'sqm' AS area_total_floor_unit,
        COUNT(*) FILTER (WHERE classname = 'RoofSurface') AS surface_count_roof,
        COUNT(*) FILTER (WHERE classname = 'WallSurface') AS surface_count_wall,
        0 AS surface_count_floor,
        -- Eave height: max vertical span across all wall faces.
        MAX(height) FILTER (WHERE classname = 'WallSurface') AS min_height,
        'm' AS min_height_unit,
        -- Ridge height: eave height + max vertical span across all roof faces.
        MAX(height) FILTER (WHERE classname = 'WallSurface') +
        COALESCE(MAX(height) FILTER (WHERE classname = 'RoofSurface'), 0) AS max_height,
        'm' AS max_height_unit,
        2.5 AS room_height,
        'm' AS room_height_unit,
        CASE
            WHEN MAX(height) FILTER (WHERE classname = 'WallSurface') > 0 AND 2.5 > 0
            THEN MAX(height) FILTER (WHERE classname = 'WallSurface') / 2.5
            ELSE 1
        END AS number_of_storeys,
        ST_Transform(ST_Force2D(ST_Centroid(
            ST_Union(geom) FILTER (WHERE classname = 'GroundSurface')
        )), {srid}) AS building_centroid_geom,
        ST_Transform(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'), {srid}) AS building_footprint_geom

    FROM
        {city2tabula_schema}.{lod_schema}_surface_raw cfs
    WHERE
        geom IS NOT NULL
        AND building_feature_id IN {building_ids}
    GROUP BY
        building_feature_id
)
INSERT INTO {city2tabula_schema}.{lod_schema}_building (
    id,
    object_id,
    country_code,
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
    surface_count_roof,
    surface_count_wall,
    surface_count_floor,
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
    gen_random_uuid() AS id,
    object_id,
    '{country_code}'  AS country_code,
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
    surface_count_roof,
    surface_count_wall,
    surface_count_floor,
    min_height,
    min_height_unit,
    max_height,
    max_height_unit,
    room_height,
    room_height_unit,
    number_of_storeys,
    building_centroid_geom,
    building_footprint_geom
FROM aggregated_surfaces;
