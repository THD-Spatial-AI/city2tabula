-- Aggregates per-surface attributes into one row per solid (_building_part) and then
-- one row per CityGML Building (_building). A Building's solids are the Building
-- itself and/or its BuildingParts (script 01). Areas use each face's exposed area,
-- surface_area minus area_internal, so faces between two solids of the building
-- (script 03) are not counted as envelope.
--
-- Height semantics, per solid, above the solid's lowest point:
--   min_height — eave height: mean of its roof faces' lowest points, weighted by
--                exposed roof area, so a small porch or courtyard roof barely moves it.
--   max_height — ridge height: highest point of its roof faces.
-- Taken from the roof, not the walls: a gable wall reaches the ridge. A solid without
-- roof faces uses the top of its walls for both.
-- A building takes the footprint-weighted mean of its solids' heights, so scripts 05
-- and 06 (height × footprint_area, footprint_area × storeys) equal the sums over its
-- solids, and a tower does not lend its height to the podium beside it. A building
-- whose solids have no ground area falls back to the tallest solid.
--
-- Complexity codes (0 = simple, 1 = regular, 2 = complex):
--   footprint_complexity — based on vertex count of the merged GroundSurface boundary.
--   roof_complexity      — based on number of distinct exposed RoofSurface polygons.
--
-- Surface counts: one per face, one per exposed piece of a partly internal face, none
-- for a fully internal one. post/04_wall_totals.sql recounts walls after party walls.
--
-- area_total_floor is the exposed GroundSurface area sum here; script 06 overwrites
-- it with the total heated floor area estimate and sets the storey counts from
-- storey_height (STOREY_HEIGHT). room_height stays at TABULA's 2.5 m reference room
-- height (h_room), which TABULA uses only for ventilation volume.
--
-- The attached-neighbour columns are left NULL; sql/scripts/post/01_detect_neighbours.sql
-- fills them once every batch has its footprints.
--
-- dataset_id is the Building feature's lineage, which the importer sets to the
-- dataset folder's dataset_id. RunFeatureExtraction checks every Building has one
-- with a dataset_attribution row before this script runs.

WITH faces AS (
    SELECT s.owner_feature_id, s.building_feature_id, s.classname, s.geom,
           ST_ZMin(s.geom) AS zmin, ST_ZMax(s.geom) AS zmax,
           s.surface_area - COALESCE(s.area_internal, 0) AS exposed,
           (s.surface_area - COALESCE(s.area_internal, 0)) / NULLIF(s.surface_area, 0) AS exposed_share
    FROM {city2tabula_schema}.{lod_schema}_surface_raw s
    WHERE s.geom IS NOT NULL
      AND s.building_feature_id IN {building_ids}
),
solids AS (
    SELECT
        owner_feature_id,
        MIN(building_feature_id) AS building_feature_id,
        ROUND(SUM(exposed) FILTER (WHERE classname = 'GroundSurface')::numeric, 2) AS footprint_area,
        MIN(zmin) AS base_z,
        COALESCE(SUM(zmin * exposed) FILTER (WHERE classname = 'RoofSurface')
                     / NULLIF(SUM(exposed) FILTER (WHERE classname = 'RoofSurface'), 0),
                 MIN(zmin) FILTER (WHERE classname = 'RoofSurface'),
                 MAX(zmax) FILTER (WHERE classname = 'WallSurface')) AS eave_z,
        COALESCE(MAX(zmax) FILTER (WHERE classname = 'RoofSurface'),
                 MAX(zmax) FILTER (WHERE classname = 'WallSurface')) AS ridge_z
    FROM faces
    GROUP BY owner_feature_id
),
-- Usable attic floor area, WoFlV § 4: plan area under the roof with at least 2 m clear
-- height counts in full, 1 to 2 m half, below 1 m not at all. A roof face is taken to
-- rise linearly from its lowest to its highest point above the eave, so the share of
-- its plan area at height h or more is (top - h) / (top - bottom), clamped to [0, 1].
attic AS (
    SELECT f.owner_feature_id,
           SUM(ST_Area(ST_Force2D(f.geom)) * COALESCE(f.exposed_share, 0)
               * (above.h2 + 0.5 * (above.h1 - above.h2))) AS area
    FROM faces f
    JOIN solids s USING (owner_feature_id)
    CROSS JOIN LATERAL (SELECT f.zmin - s.eave_z AS bottom, f.zmax - s.eave_z AS top) r
    CROSS JOIN LATERAL (
        SELECT CASE WHEN r.top > r.bottom THEN GREATEST(0, LEAST(1, (r.top - 2) / (r.top - r.bottom)))
                    ELSE (r.bottom >= 2)::int END AS h2,
               CASE WHEN r.top > r.bottom THEN GREATEST(0, LEAST(1, (r.top - 1) / (r.top - r.bottom)))
                    ELSE (r.bottom >= 1)::int END AS h1
    ) above
    WHERE f.classname = 'RoofSurface'
    GROUP BY f.owner_feature_id
)
INSERT INTO {city2tabula_schema}.{lod_schema}_building_part (
    owner_feature_id,
    owner_object_id,
    building_feature_id,
    footprint_area,
    min_height,
    max_height,
    attic_floor_area
)
SELECT
    s.owner_feature_id,
    f.objectid,
    s.building_feature_id,
    s.footprint_area,
    ROUND((s.eave_z - s.base_z)::numeric, 2),
    ROUND((s.ridge_z - s.base_z)::numeric, 2),
    ROUND(COALESCE(a.area, 0)::numeric, 2)
FROM solids s
JOIN {lod_schema}.feature f ON f.id = s.owner_feature_id
LEFT JOIN attic a USING (owner_feature_id)
ON CONFLICT (owner_feature_id) DO NOTHING;

WITH heights AS (
    SELECT
        building_feature_id,
        COALESCE(ROUND((SUM(min_height * footprint_area) /
            NULLIF(SUM(footprint_area) FILTER (WHERE min_height IS NOT NULL), 0))::numeric, 2),
            MAX(min_height)) AS min_height,
        COALESCE(ROUND((SUM(max_height * footprint_area) /
            NULLIF(SUM(footprint_area) FILTER (WHERE max_height IS NOT NULL), 0))::numeric, 2),
            MAX(max_height)) AS max_height,
        ROUND(SUM(attic_floor_area)::numeric, 2) AS attic_floor_area
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
    -- A building with ground faces only has no envelope to classify or serve, so
    -- it gets no row. Its raw surfaces stay in _surface_raw for inspection.
    HAVING COUNT(*) FILTER (WHERE cfs.classname IN ('WallSurface', 'RoofSurface')) > 0
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
    storey_height,
    storey_height_unit,
    attic_floor_area,
    attic_floor_area_unit,
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
    {storey_height} AS storey_height,
    'm' AS storey_height_unit,
    t.attic_floor_area,
    'sqm' AS attic_floor_area_unit,
    a.building_centroid_geom,
    a.building_footprint_geom
FROM aggregated_surfaces a
JOIN {lod_schema}.feature f ON f.id = a.building_feature_id
LEFT JOIN heights t ON t.building_feature_id = a.building_feature_id;
