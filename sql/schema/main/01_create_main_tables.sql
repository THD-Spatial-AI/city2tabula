DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lod INT NOT NULL,
    building_feature_id BIGINT NOT NULL,
    -- owner_feature_id: the feature owning the solid this surface bounds, the
    -- Building itself or one of its BuildingParts.
    owner_feature_id BIGINT NOT NULL,
    surface_feature_id BIGINT NOT NULL,
    building_object_id VARCHAR(100),
    surface_object_id  VARCHAR(100),
    objectclass_id INT,
    classname TEXT,
    geom GEOMETRY(MultiPolygonZ, {srid})
);


DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_geom_dump CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    child_row_id UUID,
    building_feature_id INTEGER NOT NULL,
    owner_feature_id BIGINT NOT NULL,
    surface_feature_id INTEGER NOT NULL,
    building_object_id VARCHAR(100),
    surface_object_id  VARCHAR(100),
    objectclass_id INTEGER,
    classname TEXT,
    coord_dim INT,
    has_z BOOLEAN,
    geom geometry(POLYGONZ, {srid})
);

-- Raw surface attributes computed by the pipeline.
-- Keeps building_feature_id and surface_feature_id so individual surfaces can
-- be traced back to their source features in 3DCityDB.
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_surface_raw CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_surface_raw (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  building_feature_id INTEGER,
  owner_feature_id BIGINT,
  surface_feature_id INTEGER,
  building_object_id VARCHAR(100),
  surface_object_id  VARCHAR(100),
  objectclass_id INTEGER,
  classname VARCHAR(255),
  height DOUBLE PRECISION,
  height_unit VARCHAR CHECK (height_unit IN ('m')),
  -- length / width: long and short side of the minimum-area rectangle around the
  -- face, measured in its own plane (surface_dimensions). NULL outside wall, roof and ground.
  length DOUBLE PRECISION,
  length_unit VARCHAR CHECK (length_unit IN ('m')),
  width DOUBLE PRECISION,
  width_unit VARCHAR CHECK (width_unit IN ('m')),
  surface_area DOUBLE PRECISION,
  surface_area_unit VARCHAR CHECK (surface_area_unit IN ('sqm')),
  -- tilt: angle from vertical, asin(|nz|). 0 = wall, 90 = flat roof. Complement of
  -- the usual from-horizontal slope angle; a consumer converts with 90 - tilt.
  tilt DOUBLE PRECISION,
  tilt_unit VARCHAR CHECK (tilt_unit IN ('degrees')),
  -- azimuth: compass bearing of the outward normal, clockwise from grid north. -1 = undefined,
  -- outside 0-360 so it cannot be read as a bearing. Energy ADE 1.0 uses 0 for horizontal surfaces.
  azimuth DOUBLE PRECISION,
  azimuth_unit VARCHAR CHECK (azimuth_unit IN ('degrees')),
  -- normal_x/y/z: unit normal after the orientation rules of script 03 (walls
  -- outward, roofs upward, other classes as wound in the source).
  normal_x DOUBLE PRECISION,
  normal_y DOUBLE PRECISION,
  normal_z DOUBLE PRECISION,
  -- area_internal / geom_exposed: set only on a face that lies against a face of
  -- another solid of the same building (script 03). geom_exposed is the part of
  -- the face that stays exterior, empty when the whole face is internal.
  area_internal DOUBLE PRECISION,
  geom_exposed geometry(MULTIPOLYGONZ, {srid}),
  is_valid BOOLEAN,
  is_planar BOOLEAN,
  is_party_wall BOOLEAN DEFAULT FALSE,
  neighbour_building_id INTEGER,
  child_row_id UUID,
  attribute_calc_status VARCHAR,
  geom geometry(POLYGONZ, {srid})
);

-- One row per solid of a building: the Building itself when it owns a solid, and
-- each BuildingPart that owns one. Script 04 aggregates these into _building.
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building_part CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building_part (
  owner_feature_id BIGINT PRIMARY KEY,
  owner_object_id VARCHAR(100),
  building_feature_id INTEGER NOT NULL,
  footprint_area DOUBLE PRECISION,
  min_height DOUBLE PRECISION,
  max_height DOUBLE PRECISION
);

-- Building-level attributes aggregated from surface data, one row per CityGML
-- Building however many BuildingParts carry its geometry.
-- object_id is the Building's stable 3D city model object identifier (supports both CityGML
-- and CityJSON); used as the external join key in city2tabula.building_link.
-- building_feature_id is session-local; changes on every re-import and is
-- only used as a fast join key within a single pipeline run.
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  object_id VARCHAR(100) UNIQUE,
  country_code CHAR(2),
  building_feature_id INTEGER UNIQUE,
  tabula_variant_code_id INTEGER,
  tabula_variant_code VARCHAR,
  construction_year INTEGER,
  comment TEXT,
  footprint_area DOUBLE PRECISION,
  footprint_complexity INTEGER CHECK (footprint_complexity IN (0, 1, 2)),
  roof_complexity INTEGER CHECK (roof_complexity IN (0, 1, 2)),
  has_attached_neighbour BOOLEAN,
  attached_neighbour_id INTEGER[],
  total_attached_neighbour INTEGER,
  attached_neighbour_class INTEGER CHECK (attached_neighbour_class IN (0, 1, 2, -1)),
  min_height DOUBLE PRECISION,
  min_height_unit VARCHAR(20) CHECK (min_height_unit IN ('m')),
  max_height DOUBLE PRECISION,
  max_height_unit VARCHAR(20) CHECK (max_height_unit IN ('m')),
  room_height DOUBLE PRECISION,
  room_height_unit VARCHAR(20) CHECK (room_height_unit IN ('m')),
  number_of_storeys INTEGER,
  min_volume DOUBLE PRECISION,
  min_volume_unit VARCHAR(20) CHECK (min_volume_unit IN ('cbm')),
  max_volume DOUBLE PRECISION,
  max_volume_unit VARCHAR(20) CHECK (max_volume_unit IN ('cbm')),
  area_total_roof DOUBLE PRECISION,
  area_total_roof_unit VARCHAR(20) CHECK (area_total_roof_unit IN ('sqm')),
  area_total_wall DOUBLE PRECISION,
  area_total_wall_unit VARCHAR(20) CHECK (area_total_wall_unit IN ('sqm')),
  area_total_floor DOUBLE PRECISION,
  area_total_floor_unit VARCHAR(20) CHECK (area_total_floor_unit IN ('sqm')),
  surface_count_floor INTEGER,
  surface_count_roof INTEGER,
  surface_count_wall INTEGER,
  building_centroid_geom GEOMETRY(Point, {srid}),
  building_footprint_geom GEOMETRY(MultiPolygonZ, {srid}),
  -- created_at is set once by script 04's INSERT and never touched again.
  -- updated_at only moves once the correction triggers are enabled (see
  -- 03_create_correction_triggers.sql); comparing the two tells you both
  -- whether a row has ever been hand-corrected and when it last happened.
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Resolved surface output. One row per surface polygon face, party walls excluded.
-- Populated by script 08 from lod2_surface_raw; re-run script 08 after
-- neighbour detection to apply party-wall exclusions.
-- surface_object_id / surface_feature_id are the source surface feature and are
-- shared across every face of a multi-face feature (e.g. a 3DBAG WallSurface), so
-- neither is unique in this table; the row id is the only unique key.
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_surface CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_surface (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    building_object_id VARCHAR(100) NOT NULL,
    surface_object_id  VARCHAR(100) NOT NULL,
    surface_feature_id INTEGER,
    surface_type       VARCHAR(50),
    surface_area       DOUBLE PRECISION,
    -- area_below_precision: the surface is real and its geometry, tilt and
    -- azimuth are sound, but its area rounds to 0.00 at the 2-decimal precision
    -- the pipeline records, so the area must not be used as a thermal area.
    -- Slivers from wall/roof intersections in the source model land here. The
    -- row is kept because the geometry is source data and still renders.
    area_below_precision BOOLEAN,
    -- tilt: angle from vertical, 0 = wall, 90 = flat roof. Complement of the
    -- usual from-horizontal slope angle; a consumer converts with 90 - tilt.
    tilt               DOUBLE PRECISION,
    -- azimuth: compass bearing of the outward normal, clockwise from grid north. -1 = undefined,
    -- outside 0-360 so it cannot be read as a bearing. Energy ADE 1.0 uses 0 for horizontal surfaces.
    azimuth            DOUBLE PRECISION,
    height             DOUBLE PRECISION,
    -- length / width: long and short side of the minimum-area rectangle around the
    -- face, measured in its own plane (surface_dimensions). NULL outside wall, roof and ground.
    length             DOUBLE PRECISION,
    width              DOUBLE PRECISION,
    is_valid           BOOLEAN,
    is_planar          BOOLEAN,
    is_party_wall      BOOLEAN,
    neighbour_building_id INTEGER,
    geom               GEOMETRY(POLYGONZ, {srid}),
    created_at         TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS {lod_schema}_child_geometry_idx
    ON {city2tabula_schema}.{lod_schema}_child_feature USING GIST (geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_surface_raw_geom_idx
    ON {city2tabula_schema}.{lod_schema}_surface_raw USING GIST (geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_building_part_building_idx
    ON {city2tabula_schema}.{lod_schema}_building_part (building_feature_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_raw_building_feature_id_idx
    ON {city2tabula_schema}.{lod_schema}_surface_raw (building_feature_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_raw_surface_feature_id_idx
    ON {city2tabula_schema}.{lod_schema}_surface_raw (surface_feature_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_raw_party_wall_idx
    ON {city2tabula_schema}.{lod_schema}_surface_raw (building_feature_id) WHERE is_party_wall = TRUE;

CREATE INDEX IF NOT EXISTS {lod_schema}_building_centroid_idx
    ON {city2tabula_schema}.{lod_schema}_building USING GIST (building_centroid_geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_building_footprint_idx
    ON {city2tabula_schema}.{lod_schema}_building USING GIST (building_footprint_geom);

-- Fast lookup: all surfaces for a given building
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_building_idx
    ON {city2tabula_schema}.{lod_schema}_surface (building_object_id);
-- Lookup by source surface feature (non-unique: one feature has many faces)
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_surface_idx
    ON {city2tabula_schema}.{lod_schema}_surface (surface_object_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_geom_idx
    ON {city2tabula_schema}.{lod_schema}_surface USING GIST (geom);
