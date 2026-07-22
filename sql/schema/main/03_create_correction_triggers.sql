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
--        -> tabula_variant_code / tabula_variant_code_id re-matched (mirrors script 07)
-- Step 2 fires automatically after step 1's UPDATE, because Postgres re-evaluates
-- "AFTER UPDATE OF <cols>" triggers on every UPDATE statement that touches those
-- columns, including ones issued from inside another trigger's function body.
--
-- Two more triggers cover the other correction direction — hand-editing
-- room_height or number_of_storeys directly (e.g. from a site visit or a
-- building register) instead of geometry. min_height stays fixed (it's derived
-- from wall geometry, not user-correctable here), so min_height = room_height *
-- number_of_storeys is kept true from whichever side gets edited:
--   3. room_height changes
--        -> number_of_storeys recomputed as min_height / room_height (mirrors script 06)
--   4. number_of_storeys changes (directly, or cascaded from trigger 3)
--        -> room_height recomputed as min_height / number_of_storeys (the mirror image of 3)
--        -> area_total_floor recomputed as footprint_area * number_of_storeys (mirrors script 06)
-- Trigger 4 firing is itself watched by trigger 2 (area_total_floor and
-- number_of_storeys are both variant-matching dimensions), so editing either
-- room_height or number_of_storeys directly re-matches the TABULA variant too.
-- Trigger 4's own room_height update also re-fires trigger 3, which recomputes
-- number_of_storeys right back to (within 2-decimal rounding of) what triggered
-- it in the first place — a self-check, not an infinite loop, since it settles
-- as soon as the recomputed room_height stops changing.
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
--   - has_attached_neighbour / attached_neighbour_*: neighbour detection isn't
--     implemented yet (still placeholder values from script 04), so there is nothing
--     real to invalidate. Add a trigger once that pipeline exists.
--   - Deleting a building row: no other row currently depends on it, so a plain
--     DELETE needs no trigger.
--
-- Operational note: these triggers only matter once corrections start, which is
-- always after -extract-features has already populated the tables. Bulk extraction
-- (scripts 04-07) writes the exact same watched columns these triggers watch, so
-- if the triggers were enabled during that run, every row would get redundantly
-- recomputed a second time, correctly but wastefully at 100k+ building scale, plus
-- the extra lock activity across concurrent workers risks deadlock retries. The
-- fifth (updated_at) trigger has its own reason to stay off during bulk extraction:
-- scripts 05-07 each UPDATE every row, so if it were enabled then, updated_at would
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
            WHEN number_of_storeys IS NOT NULL THEN ROUND((new_footprint_area * number_of_storeys)::numeric, 2)
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

-- Re-matches the closest TABULA variant using the same normalised-Euclidean-distance
-- method as script 07, scoped to one building instead of the whole table. Stats
-- (min/max per dimension) are still computed over the full buildings+variants table,
-- so a single-row edit is judged against the same scale as the original bulk match.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_variant_match()
RETURNS TRIGGER AS $$
DECLARE
    match RECORD;
BEGIN
    IF NEW.footprint_area IS NULL OR NEW.number_of_storeys IS NULL
       OR NEW.area_total_roof IS NULL OR NEW.area_total_wall IS NULL
       OR NEW.area_total_floor IS NULL THEN
        RETURN NEW;
    END IF;

    WITH stats AS (
        SELECT
            MIN(max_volume) AS min_vol,     MAX(max_volume) AS max_vol,
            MIN(footprint_area) AS min_area, MAX(footprint_area) AS max_area,
            MIN(number_of_storeys) AS min_storeys, MAX(number_of_storeys) AS max_storeys,
            MIN(footprint_complexity) AS min_fc, MAX(footprint_complexity) AS max_fc,
            MIN(roof_complexity) AS min_rc, MAX(roof_complexity) AS max_rc,
            MIN(area_total_roof) AS min_roof, MAX(area_total_roof) AS max_roof,
            MIN(area_total_wall) AS min_wall, MAX(area_total_wall) AS max_wall,
            MIN(area_total_floor) AS min_floor, MAX(area_total_floor) AS max_floor
        FROM (
            SELECT max_volume, footprint_area, number_of_storeys, footprint_complexity,
                   roof_complexity, area_total_roof, area_total_wall, area_total_floor
            FROM {city2tabula_schema}.{lod_schema}_building
            WHERE footprint_area IS NOT NULL AND number_of_storeys IS NOT NULL
              AND area_total_roof IS NOT NULL AND area_total_wall IS NOT NULL
              AND area_total_floor IS NOT NULL
            UNION ALL
            SELECT max_volume, footprint_area, number_of_storeys, footprint_complexity,
                   roof_complexity, area_total_roof, area_total_wall, area_total_floor
            FROM {city2tabula_schema}.tabula_variant
            WHERE max_volume IS NOT NULL AND footprint_area IS NOT NULL
              AND number_of_storeys IS NOT NULL AND area_total_roof IS NOT NULL
              AND area_total_wall IS NOT NULL AND area_total_floor IS NOT NULL
        ) all_data
    )
    SELECT v.tabula_variant_code_id, v.tabula_variant_code
    INTO match
    FROM {city2tabula_schema}.tabula_variant v
    CROSS JOIN stats s
    WHERE v.max_volume IS NOT NULL AND v.footprint_area IS NOT NULL
      AND v.number_of_storeys IS NOT NULL AND v.area_total_roof IS NOT NULL
      AND v.area_total_wall IS NOT NULL AND v.area_total_floor IS NOT NULL
    ORDER BY sqrt(
        power(COALESCE(((NEW.max_volume - s.min_vol) / NULLIF(s.max_vol - s.min_vol, 0)), 0) -
              COALESCE(((v.max_volume - s.min_vol) / NULLIF(s.max_vol - s.min_vol, 0)), 0), 2) +
        power(COALESCE(((NEW.footprint_area - s.min_area) / NULLIF(s.max_area - s.min_area, 0)), 0) -
              COALESCE(((v.footprint_area - s.min_area) / NULLIF(s.max_area - s.min_area, 0)), 0), 2) +
        power(COALESCE(((NEW.number_of_storeys - s.min_storeys) / NULLIF(s.max_storeys - s.min_storeys, 0)), 0) -
              COALESCE(((v.number_of_storeys - s.min_storeys) / NULLIF(s.max_storeys - s.min_storeys, 0)), 0), 2) +
        power(COALESCE(((NEW.footprint_complexity - s.min_fc) / NULLIF(s.max_fc - s.min_fc, 0)), 0) -
              COALESCE(((v.footprint_complexity - s.min_fc) / NULLIF(s.max_fc - s.min_fc, 0)), 0), 2) +
        power(COALESCE(((NEW.roof_complexity - s.min_rc) / NULLIF(s.max_rc - s.min_rc, 0)), 0) -
              COALESCE(((v.roof_complexity - s.min_rc) / NULLIF(s.max_rc - s.min_rc, 0)), 0), 2) +
        power(COALESCE(((NEW.area_total_roof - s.min_roof) / NULLIF(s.max_roof - s.min_roof, 0)), 0) -
              COALESCE(((v.area_total_roof - s.min_roof) / NULLIF(s.max_roof - s.min_roof, 0)), 0), 2) +
        power(COALESCE(((NEW.area_total_wall - s.min_wall) / NULLIF(s.max_wall - s.min_wall, 0)), 0) -
              COALESCE(((v.area_total_wall - s.min_wall) / NULLIF(s.max_wall - s.min_wall, 0)), 0), 2) +
        power(COALESCE(((NEW.area_total_floor - s.min_floor) / NULLIF(s.max_floor - s.min_floor, 0)), 0) -
              COALESCE(((v.area_total_floor - s.min_floor) / NULLIF(s.max_floor - s.min_floor, 0)), 0), 2)
    ) ASC
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
    AFTER UPDATE OF max_volume, footprint_area, number_of_storeys, footprint_complexity,
        roof_complexity, area_total_roof, area_total_wall, area_total_floor
    ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_variant_match();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_variant_dims_change;

-- Same formula as script 06: only overwrites number_of_storeys when both
-- min_height and the new room_height are present and positive, otherwise
-- leaves it as-is (e.g. a value already set by script 06's own fallback cascade).
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_storeys_from_room_height()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE {city2tabula_schema}.{lod_schema}_building
    SET number_of_storeys = CASE
            WHEN min_height IS NOT NULL AND NEW.room_height IS NOT NULL
                 AND NEW.room_height > 0 AND min_height > 0
            THEN min_height / NEW.room_height
            ELSE number_of_storeys
        END
    WHERE id = NEW.id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS {lod_schema}_trg_room_height_change
    ON {city2tabula_schema}.{lod_schema}_building;
CREATE TRIGGER {lod_schema}_trg_room_height_change
    AFTER UPDATE OF room_height ON {city2tabula_schema}.{lod_schema}_building
    FOR EACH ROW
    WHEN (OLD.room_height IS DISTINCT FROM NEW.room_height)
    EXECUTE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_storeys_from_room_height();
ALTER TABLE {city2tabula_schema}.{lod_schema}_building
    DISABLE TRIGGER {lod_schema}_trg_room_height_change;

-- Fires on a direct number_of_storeys edit, or one cascaded from the
-- room-height trigger above — either way, area_total_floor (the "heated floor
-- area" ignis uses as A_ref) needs to reflect the corrected storey count.
--
-- Also recomputes room_height, the mirror image of what the room-height
-- trigger does to number_of_storeys: min_height = room_height * number_of_storeys
-- must hold no matter which of the two a user edits directly (min_height itself
-- stays fixed, since it's derived from wall geometry, not user-correctable here).
-- Rounding room_height to 2 decimals (matching every other stored attribute) can
-- make the room-height trigger's own recompute land a hair off the exact value
-- just set here — e.g. 3 storeys becoming 2.99 — which is expected, not a bug:
-- the same rounding trade-off already applies in the room-height-edit direction.
CREATE OR REPLACE FUNCTION {city2tabula_schema}.{lod_schema}_recalc_floor_area_from_storeys()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE {city2tabula_schema}.{lod_schema}_building
    SET area_total_floor = CASE
            WHEN footprint_area IS NOT NULL
            THEN ROUND((footprint_area * NEW.number_of_storeys)::numeric, 2)
            ELSE area_total_floor
        END,
        area_total_floor_unit = 'sqm',
        room_height = CASE
            WHEN min_height IS NOT NULL AND NEW.number_of_storeys IS NOT NULL
                 AND NEW.number_of_storeys > 0
            THEN ROUND((min_height / NEW.number_of_storeys)::numeric, 2)
            ELSE room_height
        END,
        room_height_unit = 'm'
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
