-- Summary: Lets a user hand-correct a building's footprint geometry (e.g. in QGIS,
-- editing {lod_schema}_building.building_footprint_geom directly) and have every
-- attribute that depends on it recompute automatically, instead of re-running the
-- whole extraction pipeline.
--
-- Two triggers form a chain:
--   1. building_footprint_geom changes
--        -> footprint_area, footprint_complexity, building_centroid_geom,
--           min_volume, max_volume, area_total_floor recompute (mirrors scripts 04-06)
--   2. any of those recomputed columns changes
--        -> tabula_variant_code / tabula_variant_code_id re-matched (mirrors sql/scripts/post/02_label_buildings.sql)
-- Step 2 fires automatically after step 1's UPDATE, because Postgres re-evaluates
-- "AFTER UPDATE OF <cols>" triggers on every UPDATE statement that touches those
-- columns, including ones issued from inside another trigger's function body.
--
-- Two more triggers cover the other correction direction — hand-editing
-- storey_height or number_of_storeys directly (e.g. from a site visit or a
-- building register) instead of geometry. min_height (the eave) stays fixed, and
-- attic_storey with it, so min_height = storey_height * full_storeys is kept true
-- from whichever side gets edited:
--   3. storey_height changes
--        -> full_storeys = max(1, round(min_height / storey_height)) and
--           number_of_storeys = full_storeys + attic_storey (mirrors script 06)
--   4. number_of_storeys changes (directly, or cascaded from trigger 3)
--        -> full_storeys = number_of_storeys - attic_storey, at least 1
--        -> storey_height = min_height / full_storeys (the mirror image of 3)
--        -> area_total_floor = footprint_area * full_storeys, plus attic_floor_area
--           when attic_storey (mirrors script 06)
-- Trigger 4 firing is itself watched by trigger 2 (area_total_floor and
-- full_storeys are both variant-matching dimensions), so editing either
-- storey_height or number_of_storeys directly re-matches the TABULA variant too.
-- Trigger 4's own storey_height update also re-fires trigger 3, which recomputes
-- full_storeys right back to what triggered it in the first place — a self-check,
-- not an infinite loop, since it settles as soon as the recomputed storey_height
-- stops changing.
--
-- A fifth trigger stamps updated_at on every write that actually changes the row
-- (guarded by a WHEN clause, so a no-op UPDATE or an edit to some other column
-- doesn't count), whether it's a direct user correction or one of the
-- derived-column UPDATEs the four triggers above issue as a result. created_at is
-- set once at INSERT (script 04) and never changes, so updated_at > created_at is
-- how to tell a row was ever hand-corrected. It uses clock_timestamp() rather than
-- NOW() so the value is the real wall-clock moment of that write — NOW() is fixed
-- at transaction start, which would give every row touched by one multi-row
-- transaction (e.g. QGIS's buffered "Save Edits" across a selection) the same
-- timestamp.
--
-- Deliberately out of scope here:
--   - min_height / max_height: derived from wall/roof surface heights, not
--     something correctable from a footprint, storeys, or room-height edit.
--   - has_attached_neighbour / attached_neighbour_*: a footprint edit does not
--     re-run neighbour detection (sql/scripts/post/01_detect_neighbours.sql) for the
--     building or its neighbours. Editing attached_neighbour_class directly does
--     re-match the variant (trigger 2).
--   - Deleting a building row: no other row currently depends on it, so a plain
--     DELETE needs no trigger.
--
-- Operational note: these triggers only matter once corrections start, which is
-- always after -extract-features has already populated the tables. Bulk extraction
-- (scripts 04-06 and sql/scripts/post/) writes the exact same watched columns these triggers watch, so
-- if the triggers were enabled during that run, every row would get redundantly
-- recomputed a second time, correctly but wastefully at 100k+ building scale, plus
-- the extra lock activity across concurrent workers risks deadlock retries. The
-- fifth (updated_at) trigger has its own reason to stay off during bulk extraction:
-- scripts 05-06 and the post scripts each UPDATE every row, so if it were enabled then, updated_at would
-- already differ from created_at before any real correction ever happened, and the
-- "has this row been hand-corrected" signal would be worthless.
-- All five triggers are therefore created DISABLED below and stay that way through
-- -create-db. RunFeatureExtraction (internal/process/feature_extraction.go) turns
-- them back on itself, per LOD schema, right after -extract-features finishes —
-- no manual step needed before making a correction (e.g. in QGIS).

CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_footprint_derived()
RETURNS TRIGGER AS $$
DECLARE
    -- Rounded once here (2 decimals, matching scripts 03-06) instead of at each
    -- use below, so ST_Area is computed once and every dependent column agrees.
    new_footprint_area double precision := ROUND(ST_Area(NEW.building_footprint_geom)::numeric, 2);
BEGIN
    UPDATE {city2tabula_schema}.{lod_schema}_building
    SET
        footprint_area = new_footprint_area,
        footprint_complexity = CASE
            WHEN ST_NPoints(ST_Boundary(NEW.building_footprint_geom)) <= 4 THEN 0
            WHEN ST_NPoints(ST_Boundary(NEW.building_footprint_geom)) BETWEEN 5 AND 10 THEN 1
            ELSE 2
        END,
        building_centroid_geom = ST_Force2D(ST_Centroid(NEW.building_footprint_geom)),
        min_volume = CASE
            WHEN min_height IS NOT NULL THEN ROUND((min_height * new_footprint_area)::numeric, 2)
            ELSE min_volume
        END,
        min_volume_unit = CASE WHEN min_height IS NOT NULL THEN 'cbm' ELSE min_volume_unit END,
        max_volume = CASE
            WHEN max_height IS NOT NULL THEN ROUND((max_height * new_footprint_area)::numeric, 2)
            ELSE max_volume
        END,
        max_volume_unit = CASE WHEN max_height IS NOT NULL THEN 'cbm' ELSE max_volume_unit END,
        area_total_floor = CASE
            WHEN full_storeys IS NOT NULL THEN ROUND((new_footprint_area * full_storeys
                + CASE WHEN attic_storey THEN COALESCE(attic_floor_area, 0) ELSE 0 END)::numeric, 2)
            ELSE area_total_floor
        END,
        area_total_floor_unit = 'sqm'
    WHERE id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_footprint_geom_change
    ON {city2tabula_schema}.{lod_schema}_building;
-- Not "OLD.building_footprint_geom IS DISTINCT FROM NEW...": IS DISTINCT FROM on
-- geometry resolves PostGIS's `=` operator, which only resolves unambiguously
-- when `public` (wherever the postgis extension lives) is in the session's
-- search_path. city2tabula's own DB connection has that by default, but a
-- pg_dump --schema=city2tabula / pg_restore round-trip (e.g. sharing a corrected
-- dataset, see heat-demand-models/export-data.sh) narrows search_path to just
-- city2tabula, which makes that operator ambiguous and silently drops this
-- trigger on restore. public.ST_Equals is schema-qualified and side-steps the
-- ambiguity regardless of search_path; it also happens to be exact vertex-level
-- equality rather than the bare `=` operator's bounding-box-only comparison.
CREATE TRIGGER {lod_schema}_trg_footprint_geom_change
    AFTER UPDATE OF building_footprint_geom ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    WHEN (
        (OLD.building_footprint_geom IS NULL) IS DISTINCT FROM (NEW.building_footprint_geom IS NULL)
        OR (
            OLD.building_footprint_geom IS NOT NULL AND NEW.building_footprint_geom IS NOT NULL
            AND NOT public.ST_Equals(OLD.building_footprint_geom, NEW.building_footprint_geom)
        )
    )
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_footprint_derived();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_footprint_geom_change;

-- Re-matches the closest TABULA variant with the same distance as
-- post/02_label_buildings.sql, scoped to one building. The min/max per dimension is
-- still taken over all buildings and variants, so an edit is judged on the same scale
-- as the bulk match.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_variant_match()
RETURNS TRIGGER AS $$
DECLARE
    match RECORD;
BEGIN
    IF NEW.footprint_area IS NULL OR NEW.full_storeys IS NULL
       OR NEW.area_total_roof IS NULL OR NEW.area_total_wall IS NULL
       OR NEW.area_total_floor IS NULL THEN
        RETURN NEW;
    END IF;

    WITH stats AS (
        SELECT
            MIN(max_volume) AS lo_vol, MAX(max_volume) AS hi_vol,
            MIN(footprint_area) AS lo_area, MAX(footprint_area) AS hi_area,
            MIN(number_of_storeys) AS lo_storeys, MAX(number_of_storeys) AS hi_storeys,
            MIN(footprint_complexity) AS lo_fc, MAX(footprint_complexity) AS hi_fc,
            MIN(roof_complexity) AS lo_rc, MAX(roof_complexity) AS hi_rc,
            MIN(attached_neighbour_class) AS lo_ac, MAX(attached_neighbour_class) AS hi_ac,
            MIN(area_total_roof) AS lo_roof, MAX(area_total_roof) AS hi_roof,
            MIN(area_total_wall) AS lo_wall, MAX(area_total_wall) AS hi_wall,
            MIN(area_total_floor) AS lo_floor, MAX(area_total_floor) AS hi_floor
        FROM (
            SELECT max_volume, footprint_area, full_storeys AS number_of_storeys, footprint_complexity, roof_complexity, attached_neighbour_class, area_total_roof, area_total_wall, area_total_floor
            FROM {city2tabula_schema}.{lod_schema}_building
            WHERE footprint_area IS NOT NULL AND full_storeys IS NOT NULL
              AND area_total_roof IS NOT NULL AND area_total_wall IS NOT NULL
              AND area_total_floor IS NOT NULL
            UNION ALL
            SELECT max_volume, footprint_area, number_of_storeys, footprint_complexity, roof_complexity, attached_neighbour_class, area_total_roof, area_total_wall, area_total_floor
            FROM {city2tabula_schema}.tabula_variant
        ) all_data
    )
    SELECT v.tabula_variant_code_id, v.tabula_variant_code
    INTO match
    FROM {city2tabula_schema}.tabula_variant v
    CROSS JOIN stats s
    CROSS JOIN LATERAL (
        SELECT sqrt(avg(term)) AS distance
        FROM (VALUES
            (power({city2tabula_schema}.minmax_norm(NEW.max_volume, s.lo_vol, s.hi_vol)
                 - {city2tabula_schema}.minmax_norm(v.max_volume, s.lo_vol, s.hi_vol), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.footprint_area, s.lo_area, s.hi_area)
                 - {city2tabula_schema}.minmax_norm(v.footprint_area, s.lo_area, s.hi_area), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.full_storeys, s.lo_storeys, s.hi_storeys)
                 - {city2tabula_schema}.minmax_norm(v.number_of_storeys, s.lo_storeys, s.hi_storeys), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.footprint_complexity, s.lo_fc, s.hi_fc)
                 - {city2tabula_schema}.minmax_norm(v.footprint_complexity, s.lo_fc, s.hi_fc), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.roof_complexity, s.lo_rc, s.hi_rc)
                 - {city2tabula_schema}.minmax_norm(v.roof_complexity, s.lo_rc, s.hi_rc), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.attached_neighbour_class, s.lo_ac, s.hi_ac)
                 - {city2tabula_schema}.minmax_norm(v.attached_neighbour_class, s.lo_ac, s.hi_ac), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.area_total_roof, s.lo_roof, s.hi_roof)
                 - {city2tabula_schema}.minmax_norm(v.area_total_roof, s.lo_roof, s.hi_roof), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.area_total_wall, s.lo_wall, s.hi_wall)
                 - {city2tabula_schema}.minmax_norm(v.area_total_wall, s.lo_wall, s.hi_wall), 2)),
            (power({city2tabula_schema}.minmax_norm(NEW.area_total_floor, s.lo_floor, s.hi_floor)
                 - {city2tabula_schema}.minmax_norm(v.area_total_floor, s.lo_floor, s.hi_floor), 2))
        ) terms(term)
    ) d
    WHERE d.distance IS NOT NULL
    ORDER BY d.distance, v.tabula_variant_code_id
    LIMIT 1;

    IF match.tabula_variant_code_id IS NOT NULL THEN
        UPDATE {city2tabula_schema}.{lod_schema}_building
        SET tabula_variant_code_id = match.tabula_variant_code_id,
            tabula_variant_code = match.tabula_variant_code
        WHERE id = NEW.id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_variant_dims_change
    ON {city2tabula_schema}.{lod_schema}_building;
CREATE TRIGGER {lod_schema}_trg_variant_dims_change
    AFTER UPDATE OF max_volume, footprint_area, full_storeys, footprint_complexity,
        roof_complexity, attached_neighbour_class, area_total_roof, area_total_wall, area_total_floor
    ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_variant_match();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_variant_dims_change;

-- Same rule as script 06. A storey height that is missing or not positive leaves the
-- counts as they are.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_storeys_from_storey_height()
RETURNS TRIGGER AS $$
DECLARE
    full_count integer;
BEGIN
    IF NEW.min_height IS NULL OR NEW.min_height <= 0
       OR NEW.storey_height IS NULL OR NEW.storey_height <= 0 THEN
        RETURN NEW;
    END IF;
    full_count := GREATEST(1, ROUND(NEW.min_height / NEW.storey_height))::integer;
    UPDATE {city2tabula_schema}.{lod_schema}_building
    SET full_storeys = full_count,
        number_of_storeys = full_count + COALESCE(attic_storey, FALSE)::integer
    WHERE id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_storey_height_change
    ON {city2tabula_schema}.{lod_schema}_building;
CREATE TRIGGER {lod_schema}_trg_storey_height_change
    AFTER UPDATE OF storey_height ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    WHEN (OLD.storey_height IS DISTINCT FROM NEW.storey_height)
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_storeys_from_storey_height();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_storey_height_change;

-- Fires on a direct number_of_storeys edit, or one cascaded from the storey-height
-- trigger above. The attic storey is geometric, so the edit changes the full storeys:
-- full_storeys = number_of_storeys - attic_storey, at least 1. area_total_floor (the
-- heated floor area ignis uses as A_ref) and storey_height follow, so that
-- min_height = storey_height * full_storeys holds from this side too. Rounding
-- storey_height to 2 decimals can make the storey-height trigger's recompute land on
-- the same full_storeys again, which ends the cascade.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_floor_area_from_storeys()
RETURNS TRIGGER AS $$
DECLARE
    full_count integer;
BEGIN
    IF NEW.number_of_storeys IS NULL THEN
        RETURN NEW;
    END IF;
    full_count := GREATEST(1, NEW.number_of_storeys - COALESCE(NEW.attic_storey, FALSE)::integer);
    UPDATE {city2tabula_schema}.{lod_schema}_building
    SET full_storeys = full_count,
        area_total_floor = CASE
            WHEN footprint_area IS NOT NULL
            THEN ROUND((footprint_area * full_count
                + CASE WHEN attic_storey THEN COALESCE(attic_floor_area, 0) ELSE 0 END)::numeric, 2)
            ELSE area_total_floor
        END,
        area_total_floor_unit = 'sqm',
        storey_height = CASE
            WHEN min_height IS NOT NULL AND min_height > 0
            THEN ROUND((min_height / full_count)::numeric, 2)
            ELSE storey_height
        END,
        storey_height_unit = 'm'
    WHERE id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_storeys_change
    ON {city2tabula_schema}.{lod_schema}_building;
CREATE TRIGGER {lod_schema}_trg_storeys_change
    AFTER UPDATE OF number_of_storeys ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    WHEN (OLD.number_of_storeys IS DISTINCT FROM NEW.number_of_storeys)
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_floor_area_from_storeys();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_storeys_change;

-- Stamps updated_at on any write to a building row that actually changes it.
-- BEFORE UPDATE (not AFTER) so the timestamp lands in the same row version being
-- written, no follow-up UPDATE needed. Fires for a direct user edit and for every
-- cascade UPDATE the four triggers above issue, so updated_at always reflects the
-- most recent real change from any source.
--
-- Not templated per {lod_schema} like the four functions above: this body has no
-- schema-specific reference (just NEW.updated_at), so it's defined once under
-- {city2tabula_schema} and shared by both lod2's and lod3's triggers — the same
-- pattern sql/functions/01_surface_area_corrected_geom.sql already uses for a
-- schema-generic function. CREATE OR REPLACE makes re-running this script once per
-- configured LOD schema harmless (redefines the same function identically).
CREATE OR REPLACE FUNCTION {city2tabula_schema}.touch_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_touch_updated_at
    ON {city2tabula_schema}.{lod_schema}_building;
-- WHEN diffs the row as jsonb rather than comparing NEW/OLD directly: a bare
-- IS DISTINCT FROM on a row containing geometry columns resolves PostGIS's `=`
-- operator per column, which is exactly the search_path-dependent ambiguity
-- trg_footprint_geom_change's own WHEN clause (above) had to work around with an
-- explicit public.ST_Equals call. to_jsonb serializes every column via its output
-- function instead, so it never invokes that operator. Excluding updated_at and
-- created_at keeps the comparison from being trivially true on its own columns;
-- diffing generically (not a hardcoded column list) means it stays correct if a
-- new correctable column is ever added to _building.
CREATE TRIGGER {lod_schema}_trg_touch_updated_at
    BEFORE UPDATE ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    WHEN (
        (to_jsonb(NEW) - ARRAY['updated_at', 'created_at']::text[])
        IS DISTINCT FROM
        (to_jsonb(OLD) - ARRAY['updated_at', 'created_at']::text[])
    )
    EXECUTE FUNCTION {city2tabula_schema}.touch_updated_at();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_touch_updated_at;
