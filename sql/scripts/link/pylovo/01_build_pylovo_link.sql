-- Populates city2tabula.building_link by spatially joining 3D building footprints
-- against pylovo.res (residential) and pylovo.oth (commercial/public/industrial).
-- Invoked by the -link-pylovo flag.
--
-- Runs per batch of {building_ids}. For each batch:
--   1. Compute a bounding box from the batch footprints (batch_bbox).
--   2. Pre-filter pylovo.res / pylovo.oth to only buildings within that bbox
--      and pre-transform their geometry to the native 3D CRS ({srid}).
--   3. IoU join against the subset only — avoids scanning the full PyLovo table.
--
-- Match confidence = intersection area / area of the smaller footprint (IoU proxy).
-- Threshold: >= 0.5 → match_type 1 (complete), < 0.5 → no OSM match → match_type 2.
--
-- Existing rows for buildings in {building_ids} are deleted and re-inserted so
-- this script is safe to re-run after updated PyLovo data.

DELETE FROM {city2tabula_schema}.building_link
WHERE object_id IN (
    SELECT object_id
    FROM {city2tabula_schema}.{lod_schema}_building
    WHERE building_feature_id IN {building_ids}
);

WITH buildings AS (
    SELECT
        b.object_id,
        b.country_code,
        b.building_footprint_geom                        AS geom_native,
        ST_Area(ST_Force2D(b.building_footprint_geom))   AS area_3d
    FROM {city2tabula_schema}.{lod_schema}_building b
    WHERE b.building_feature_id IN {building_ids}
      AND b.building_footprint_geom IS NOT NULL
      AND b.object_id IS NOT NULL
),

batch_bbox AS (
    -- Bounding box of this batch in EPSG:3035 (PyLovo native CRS) used to
    -- pre-filter PyLovo tables before the IoU join.
    SELECT ST_Transform(
        ST_Envelope(ST_Collect(geom_native)),
        3035
    ) AS geom_3035
    FROM buildings
),

res_subset AS (
    -- PyLovo residential buildings within the batch bbox, pre-transformed to
    -- native 3D CRS so no per-row ST_Transform inside the IoU calculation.
    SELECT r.osm_id, ST_Transform(r.geom, {srid}) AS geom_native
    FROM {pylovo_schema}.res r, batch_bbox
    WHERE ST_Intersects(r.geom, batch_bbox.geom_3035)
),

oth_subset AS (
    -- Same pre-filter for commercial/public/industrial buildings.
    SELECT o.osm_id, ST_Transform(o.geom, {srid}) AS geom_native
    FROM {pylovo_schema}.oth o, batch_bbox
    WHERE ST_Intersects(o.geom, batch_bbox.geom_3035)
),

res_candidates AS (
    -- Best matching res row per 3D building: highest IoU above threshold.
    SELECT DISTINCT ON (b.object_id)
        b.object_id,
        b.country_code,
        b.geom_native,
        b.area_3d,
        r.osm_id,
        'res'                                                                     AS pylovo_table,
        ST_Area(ST_Intersection(b.geom_native, r.geom_native))
          / NULLIF(LEAST(b.area_3d, ST_Area(r.geom_native)), 0)                  AS confidence
    FROM buildings b
    JOIN res_subset r ON ST_Intersects(b.geom_native, r.geom_native)
    ORDER BY b.object_id, confidence DESC
),

oth_candidates AS (
    -- Best matching oth row per 3D building: only considered when no res match.
    SELECT DISTINCT ON (b.object_id)
        b.object_id,
        b.country_code,
        b.geom_native,
        b.area_3d,
        o.osm_id,
        'oth'                                                                     AS pylovo_table,
        ST_Area(ST_Intersection(b.geom_native, o.geom_native))
          / NULLIF(LEAST(b.area_3d, ST_Area(o.geom_native)), 0)                  AS confidence
    FROM buildings b
    JOIN oth_subset o ON ST_Intersects(b.geom_native, o.geom_native)
    WHERE b.object_id NOT IN (
        SELECT object_id FROM res_candidates WHERE confidence >= 0.5
    )
    ORDER BY b.object_id, confidence DESC
),

matched AS (
    -- Prefer res over oth; apply confidence threshold.
    SELECT object_id, country_code, geom_native, osm_id, pylovo_table, confidence
    FROM res_candidates
    WHERE confidence >= 0.5
    UNION ALL
    SELECT object_id, country_code, geom_native, osm_id, pylovo_table, confidence
    FROM oth_candidates
    WHERE confidence >= 0.5
),

unmatched AS (
    -- Buildings with no OSM match above threshold → match_type 2 (3D only).
    SELECT b.object_id, b.country_code, b.geom_native
    FROM buildings b
    WHERE b.object_id NOT IN (SELECT object_id FROM matched)
)

INSERT INTO {city2tabula_schema}.building_link
    (object_id, osm_id, pylovo_table, match_type, match_confidence, country_code, geom, srid)
SELECT
    object_id,
    osm_id,
    pylovo_table,
    1                       AS match_type,
    confidence              AS match_confidence,
    country_code,
    ST_Force2D(geom_native) AS geom,
    {srid}::INTEGER         AS srid
FROM matched
UNION ALL
SELECT
    object_id,
    NULL                    AS osm_id,
    NULL                    AS pylovo_table,
    2                       AS match_type,
    NULL                    AS match_confidence,
    country_code,
    ST_Force2D(geom_native) AS geom,
    {srid}::INTEGER         AS srid
FROM unmatched
ON CONFLICT (object_id, country_code) DO UPDATE
    SET osm_id           = EXCLUDED.osm_id,
        pylovo_table     = EXCLUDED.pylovo_table,
        match_type       = EXCLUDED.match_type,
        match_confidence = EXCLUDED.match_confidence,
        geom             = EXCLUDED.geom,
        created_at       = NOW();
