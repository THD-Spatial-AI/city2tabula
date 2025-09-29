-- This script populates the {lod_schema}_building_feature and lod3_building_feature tables by aggregating data from child features.
-- It excludes buildings that have already been processed to avoid unnecessary work.
-- The script calculates building footprint area, roof complexity, and centroid.

WITH new_buildings AS (
    -- Select only buildings that haven't been processed yet
    SELECT DISTINCT building_feature_id
    FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
    WHERE building_feature_id IN {building_ids}
),
building_data AS (
    SELECT
        cfs.building_feature_id,
        SUM(surface_area) FILTER (WHERE classname = 'GroundSurface') AS footprint_area,
        CASE
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) <= 4 THEN 0 -- 'simple'
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) BETWEEN 5 AND 10 THEN 1 -- 'regular'
            ELSE 2 -- 'complex'
        END AS footprint_complexity,
        CASE
            WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') = 1 THEN 0 -- 'simple'
            WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') BETWEEN 2 AND 4 THEN 1 -- 'regular'
            ELSE 2 -- 'complex'
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
        MAX(height) FILTER (WHERE classname = 'WallSurface') AS min_height,
        'm' AS min_height_unit,
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
        ST_Transform(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'), {srid}) AS  building_footprint_geom

    FROM
        {city2tabula_schema}.{lod_schema}_child_feature_surface cfs
    WHERE
        geom IS NOT NULL
        AND building_feature_id IN {building_ids}
    GROUP BY
        building_feature_id
)
INSERT INTO {city2tabula_schema}.{lod_schema}_building_feature (
    id,
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
    building_footprint_geom)
SELECT
    gen_random_uuid() AS id,
    *
FROM building_data;