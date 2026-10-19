-- Populates city2tabula.building_link by spatially joining 3D building footprints
-- against pylovo.res (residential) and pylovo.oth (commercial/public/industrial).
-- Invoked by the -link-pylovo flag.
--
-- Runs per batch of {building_ids}. For each batch:
--   1. Compute a bounding box from the batch footprints (batch_bbox).
--   2. Pre-filter pylovo.res / pylovo.oth to this country's buildings within that
--      bbox and pre-transform their geometry to the native 3D CRS ({srid}).
--   3. IoU join against the subset only — avoids scanning the full PyLovo table.
--
-- Match confidence = intersection area / area of the smaller footprint (IoU proxy).
-- Every OSM way with confidence >= 0.5 is linked, because OSM may map one 3D building
-- as several ways. The best way is match_type 1: res before oth, then the higher
-- confidence to two decimals (exact copies differ only by reprojection noise), then
-- the larger overlap, then osm_id. Each further way is match_type 4, pointing at the
-- same object_id. A building with no way >= 0.5 gets one match_type 2 row (3D only).
--
-- Existing rows for buildings in {building_ids} are deleted and re-inserted.
-- -link-pylovo batches only buildings without a row; -link-pylovo -relink batches
-- every building, which re-links them after the PyLovo data changed.
--
-- When {pylovo_schema} is a postgres_fdw foreign schema (PYLOVO_FDW_HOST set), the
-- pre-filter must reach PyLovo, which holds every country in one table. The bbox
-- is read through a scalar subquery so postgres_fdw sends it as a bound parameter
-- (a join against batch_bbox is not shippable), and country_code is a literal.

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
        b.building_footprint_geom AS geom_native,
        ST_Area(ST_Force2D(b.building_footprint_geom)) AS area_3d
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
    FROM {pylovo_schema}.res r
    WHERE r.country_code = '{country_code}'
      AND ST_Intersects(r.geom, (SELECT geom_3035 FROM batch_bbox))
),

oth_subset AS (
    -- Same pre-filter for commercial/public/industrial buildings.
    SELECT o.osm_id, ST_Transform(o.geom, {srid}) AS geom_native
    FROM {pylovo_schema}.oth o
    WHERE o.country_code = '{country_code}'
      AND ST_Intersects(o.geom, (SELECT geom_3035 FROM batch_bbox))
),

candidates AS (
    -- Every way overlapping a 3D building, with its confidence and overlap area.
    SELECT b.object_id, b.country_code, b.geom_native, w.osm_id, w.pylovo_table,
           c.overlap / NULLIF(LEAST(b.area_3d, ST_Area(w.geom_native)), 0) AS confidence,
           c.overlap
    FROM buildings b
    JOIN (
        SELECT osm_id, 'res' AS pylovo_table, geom_native FROM res_subset
        UNION ALL
        SELECT osm_id, 'oth', geom_native FROM oth_subset
    ) w ON ST_Intersects(b.geom_native, w.geom_native)
    CROSS JOIN LATERAL (
        SELECT ST_Area(ST_Intersection(b.geom_native, w.geom_native)) AS overlap
    ) c
),

matched AS (
    -- Ways above the threshold, ranked per 3D building; rank 1 is the best way.
    SELECT object_id, country_code, geom_native, osm_id, pylovo_table, confidence,
           ROW_NUMBER() OVER (
               PARTITION BY object_id
               ORDER BY pylovo_table = 'res' DESC, round(confidence::numeric, 2) DESC,
                        overlap DESC, osm_id
           ) AS way_rank
    FROM candidates
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
    CASE WHEN way_rank = 1 THEN 1 ELSE 4 END AS match_type,
    confidence AS match_confidence,
    country_code,
    ST_Force2D(geom_native) AS geom,
    {srid}::INTEGER AS srid
FROM matched
UNION ALL
SELECT
    object_id,
    NULL AS osm_id,
    NULL AS pylovo_table,
    2 AS match_type,
    NULL AS match_confidence,
    country_code,
    ST_Force2D(geom_native) AS geom,
    {srid}::INTEGER AS srid
FROM unmatched;
