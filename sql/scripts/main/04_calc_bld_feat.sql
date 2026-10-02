-- Aggregates per-surface attributes into one row per solid (_building_part) and then
-- one row per CityGML Building (_building). A Building's solids are the Building
-- itself and/or its BuildingParts (script 01). Areas use each face's exposed area,
-- surface_area minus area_internal, so faces between two solids of the building
-- (script 03) are not counted as envelope.
--
-- Height semantics, per solid (both derived from child surface heights):
--   min_height — maximum vertical span of any WallSurface face (eave height).
--                Named "min" because it excludes the roof ridge contribution.
--   max_height — eave height + maximum vertical span of any RoofSurface face (ridge height).
-- A building takes the footprint-weighted mean of its solids' heights, so scripts 05
-- and 06 (height × footprint_area, footprint_area × storeys) equal the sums over its
-- solids, and a tower does not lend its height to the podium beside it. A building
-- whose solids have no ground area falls back to the tallest solid.
--
-- Complexity codes (0 = simple, 1 = regular, 2 = complex):
--   footprint_complexity — based on vertex count of the merged GroundSurface boundary.
--   roof_complexity      — based on number of distinct exposed RoofSurface polygons.
--
-- Surface counts are the rows script 08 serves: one per face, or one per exposed
-- piece of a partly internal face, none for a fully internal one.
--
-- area_total_floor is the exposed GroundSurface area sum here; script 06 overwrites
-- it with the total heated floor area estimate.
--
-- dataset_id is the Building feature's lineage, which the importer sets to the
-- dataset folder's dataset_id. RunFeatureExtraction checks every Building has one
-- with a dataset_attribution row before this script runs.

INSERT INTO {city2tabula_schema}.{lod_schema}_building_part (
    owner_feature_id,
    owner_object_id,
    building_feature_id,
    footprint_area,
    min_height,
    max_height
)
SELECT
    s.owner_feature_id,
    f.objectid,
    MIN(s.building_feature_id),
    ROUND(SUM(s.surface_area - COALESCE(s.area_internal, 0))
          FILTER (WHERE s.classname = 'GroundSurface')::numeric, 2),
    MAX(s.height) FILTER (WHERE s.classname = 'WallSurface'),
    ROUND((MAX(s.height) FILTER (WHERE s.classname = 'WallSurface') +
           COALESCE(MAX(s.height) FILTER (WHERE s.classname = 'RoofSurface'), 0))::numeric, 2)
FROM {city2tabula_schema}.{lod_schema}_surface_raw s
JOIN {lod_schema}.feature f ON f.id = s.owner_feature_id
WHERE s.geom IS NOT NULL
  AND s.building_feature_id IN {building_ids}
GROUP BY s.owner_feature_id, f.objectid
ON CONFLICT (owner_feature_id) DO NOTHING;

WITH heights AS (
    SELECT
        building_feature_id,
        COALESCE(ROUND((SUM(min_height * footprint_area) /
            NULLIF(SUM(footprint_area) FILTER (WHERE min_height IS NOT NULL), 0))::numeric, 2),
            MAX(min_height)) AS min_height,
        COALESCE(ROUND((SUM(max_height * footprint_area) /
            NULLIF(SUM(footprint_area) FILTER (WHERE max_height IS NOT NULL), 0))::numeric, 2),
            MAX(max_height)) AS max_height
    FROM {city2tabula_schema}.{lod_schema}_building_part
    WHERE building_feature_id IN {building_ids}
    GROUP BY building_feature_id
),
faces AS (
    SELECT
        building_feature_id,
        building_object_id,
        classname,
        geom,
        surface_area - COALESCE(area_internal, 0) AS exposed_area,
        CASE WHEN geom_exposed IS NULL THEN 1 ELSE ST_NumGeometries(geom_exposed) END AS pieces
    FROM {city2tabula_schema}.{lod_schema}_surface_raw
    WHERE geom IS NOT NULL
      AND building_feature_id IN {building_ids}
),
aggregated_surfaces AS (
    SELECT
        cfs.building_feature_id,
        MIN(cfs.building_object_id) AS object_id,
        0 as construction_year,
        -- Rounded to 2 decimals: inputs are already 2-decimal (script 03), but
        -- SUM/addition of several such values can reintroduce float noise past
        -- the 2nd decimal, so every aggregate below is rounded again on the way out.
        ROUND(SUM(exposed_area) FILTER (WHERE classname = 'GroundSurface')::numeric, 2) AS footprint_area,
        -- Footprint complexity: vertex count of the merged ground boundary.
        -- ≤ 4 vertices → simple rectangle; 5–10 → regular polygon; > 10 → complex shape.
        CASE
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) <= 4 THEN 0
            WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) BETWEEN 5 AND 10 THEN 1
            ELSE 2
        END AS footprint_complexity,
        -- Roof complexity: number of distinct exposed RoofSurface polygons.
        -- 1 face → simple (flat or single-pitch); 2–4 → regular (gable, hip); > 4 → complex.
        CASE
            WHEN SUM(pieces) FILTER (WHERE classname = 'RoofSurface') = 1 THEN 0
            WHEN SUM(pieces) FILTER (WHERE classname = 'RoofSurface') BETWEEN 2 AND 4 THEN 1
            ELSE 2
        END AS roof_complexity,
        FALSE AS has_attached_neighbour,
        ARRAY[]::INTEGER[] AS attached_neighbour_id,
        0 AS total_attached_neighbour,
        ROUND(SUM(CASE WHEN classname = 'RoofSurface' THEN exposed_area ELSE 0 END)::numeric, 2) AS area_total_roof,
        'sqm' AS area_total_roof_unit,
        ROUND(SUM(CASE WHEN classname = 'WallSurface' THEN exposed_area ELSE 0 END)::numeric, 2) AS area_total_wall,
        'sqm' AS area_total_wall_unit,
        ROUND(SUM(CASE WHEN classname = 'GroundSurface' THEN exposed_area ELSE 0 END)::numeric, 2) AS area_total_floor,
        'sqm' AS area_total_floor_unit,
        COALESCE(SUM(pieces) FILTER (WHERE classname = 'RoofSurface'), 0) AS surface_count_roof,
        COALESCE(SUM(pieces) FILTER (WHERE classname = 'WallSurface'), 0) AS surface_count_wall,
        COALESCE(SUM(pieces) FILTER (WHERE classname = 'GroundSurface'), 0) AS surface_count_floor,
        ST_Transform(ST_Force2D(ST_Centroid(
            ST_Union(geom) FILTER (WHERE classname = 'GroundSurface')
        )), {srid}) AS building_centroid_geom,
        ST_Transform(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'), {srid}) AS building_footprint_geom
    FROM faces cfs
    GROUP BY cfs.building_feature_id
)
INSERT INTO {city2tabula_schema}.{lod_schema}_building (
    id,
    object_id,
    country_code,
    dataset_id,
    building_feature_id,
    construction_year,
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
    a.object_id,
    '{country_code}'  AS country_code,
    f.lineage         AS dataset_id,
    a.building_feature_id,
    a.construction_year,
    a.footprint_area,
    a.footprint_complexity,
    a.roof_complexity,
    a.has_attached_neighbour,
    a.attached_neighbour_id,
    a.total_attached_neighbour,
    a.area_total_roof,
    a.area_total_roof_unit,
    a.area_total_wall,
    a.area_total_wall_unit,
    a.area_total_floor,
    a.area_total_floor_unit,
    a.surface_count_roof,
    a.surface_count_wall,
    a.surface_count_floor,
    t.min_height,
    'm' AS min_height_unit,
    t.max_height,
    'm' AS max_height_unit,
    2.5 AS room_height,
    'm' AS room_height_unit,
    CASE WHEN t.min_height > 0 THEN t.min_height / 2.5 ELSE 1 END AS number_of_storeys,
    a.building_centroid_geom,
    a.building_footprint_geom
FROM aggregated_surfaces a
JOIN {lod_schema}.feature f ON f.id = a.building_feature_id
LEFT JOIN heights t ON t.building_feature_id = a.building_feature_id;
