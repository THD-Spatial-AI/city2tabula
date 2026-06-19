-- Bridge table linking 3D city model buildings to OSM buildings (pylovo.res / pylovo.oth).
-- Populated by the -build-link pipeline step after feature extraction.
--
-- object_id — stable 3D city model identifier (supports CityGML and CityJSON).
--             References {lod_schema}_building.object_id.
-- osm_id    — NULL when no OSM building was matched (match_type = 2: 3D only).
-- pylovo_table — which PyLovo table the OSM match came from ('res' or 'oth');
--               NULL for unmatched 3D-only buildings (defaults to 'res' treatment).
-- match_type:
--   1 = complete match (both 3D and OSM present)
--   2 = 3D only (no OSM match found above threshold)
--   3 = OSM only (no 3D match; populated from res/oth directly, not via this pipeline)
-- match_confidence — IoU score: intersection area / area of the smaller footprint.

DROP TABLE IF EXISTS {city2tabula_schema}.building_link CASCADE;
CREATE TABLE {city2tabula_schema}.building_link (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    object_id         VARCHAR(100)  NOT NULL,
    osm_id            TEXT,
    pylovo_table      VARCHAR(3)    CHECK (pylovo_table IN ('res', 'oth')),
    match_type        SMALLINT      NOT NULL CHECK (match_type IN (1, 2, 3)),
    match_confidence  DOUBLE PRECISION,
    country_code      CHAR(2)       NOT NULL,
    geom              GEOMETRY(MultiPolygon),
    srid              INTEGER       NOT NULL,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (object_id, country_code)
);

CREATE INDEX IF NOT EXISTS idx_building_link_geom
    ON {city2tabula_schema}.building_link USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_building_link_osm_id
    ON {city2tabula_schema}.building_link (osm_id);
CREATE INDEX IF NOT EXISTS idx_building_link_country
    ON {city2tabula_schema}.building_link (country_code);
CREATE INDEX IF NOT EXISTS idx_building_link_match_type
    ON {city2tabula_schema}.building_link (match_type);

-- Registry mapping (country_code, state_code) to the City2TABULA schema name
-- and current pipeline status. No geometry stored here — spatial extent is in
-- pylovo.postcode_result.geom.

DROP TABLE IF EXISTS {city2tabula_schema}.pipeline_status CASCADE;
CREATE TABLE {city2tabula_schema}.pipeline_status (
    country_code    CHAR(2)      NOT NULL,
    state_code      TEXT         NOT NULL,
    schema_name     TEXT         NOT NULL,
    last_processed  TIMESTAMPTZ,
    status          TEXT         NOT NULL DEFAULT 'ready'
                                 CHECK (status IN ('ready', 'processing', 'failed')),
    PRIMARY KEY (country_code, state_code)
);
