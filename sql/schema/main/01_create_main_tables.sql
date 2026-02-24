-- ============================================================
-- 1) Candidate mapping table (dirty / many-to-many)
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lod INT NOT NULL,
    building_feature_id BIGINT NOT NULL,
    surface_feature_id BIGINT NOT NULL,
    objectclass_id INT,
    classname TEXT,
    geom GEOMETRY(MultiPolygonZ, {srid})
);

-- Prevent duplicate candidate edges
CREATE UNIQUE INDEX IF NOT EXISTS ux_{lod_schema}_child_feature_pair
  ON {city2tabula_schema}.{lod_schema}_child_feature (lod, building_feature_id, surface_feature_id);



-- ============================================================
-- 2) Dumped geometry table (per polygon part)
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_geom_dump CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    child_row_id UUID,
    building_feature_id BIGINT NOT NULL,
    surface_feature_id BIGINT NOT NULL,
    objectclass_id INT,
    classname TEXT,
    coord_dim INT,
    has_z BOOLEAN,
    geom geometry(POLYGONZ, {srid})
);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_geom_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump USING GIST (geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_building_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (building_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_surface_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (surface_feature_id);



-- ============================================================
-- 3) Computed surface attributes table (per dumped polygon)
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_surface CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_surface (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  building_feature_id BIGINT,
  surface_feature_id BIGINT,
  objectclass_id INT,
  classname VARCHAR(255),

  height DOUBLE PRECISION,
  height_unit VARCHAR CHECK (height_unit IN ('m')),

  surface_area DOUBLE PRECISION,
  surface_area_unit VARCHAR CHECK (surface_area_unit IN ('sqm')),

  tilt DOUBLE PRECISION,
  tilt_unit VARCHAR CHECK (tilt_unit IN ('degrees')),

  azimuth DOUBLE PRECISION,
  azimuth_unit VARCHAR CHECK (azimuth_unit IN ('degrees')),

  is_valid BOOLEAN,
  is_planar BOOLEAN,

  child_row_id UUID,
  attribute_calc_status VARCHAR,

  geom geometry(POLYGONZ, {srid})
);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_geom_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_surface USING GIST (geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_building_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_surface (building_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_surface_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_surface (surface_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_childrow_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_surface (child_row_id);



-- ============================================================
-- 4) RESOLVED mapping table (clean / one owner per surface_feature_id)
--    This is what downstream analysis should use.
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_resolved CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_resolved (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  lod INT NOT NULL,

  -- surface is unique at this stage
  surface_feature_id BIGINT NOT NULL,

  -- chosen owner building
  building_feature_id BIGINT NOT NULL,

  objectclass_id INT,
  classname TEXT,

  -- assignment confidence
  score DOUBLE PRECISION,
  scoring_method VARCHAR(30),  -- e.g. 'wall_3d_ratio', 'roof_2d_ratio'

  -- convenience flag: true if this surface participates in a party-wall relation
  is_party_wall BOOLEAN DEFAULT FALSE,

  geom GEOMETRY(MultiPolygonZ, {srid})
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_{lod_schema}_child_feature_resolved_surface
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (lod, surface_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_resolved_building_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (building_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_resolved_geom_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved USING GIST (geom);



-- ============================================================
-- 5) PARTY WALL RELATION table (A <-> B link)
--    Stores which buildings share walls and the shared area.
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_party_wall CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_party_wall (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  lod INT NOT NULL,

  -- building pair (ordered to avoid duplicates)
  building_a_id BIGINT NOT NULL,
  building_b_id BIGINT NOT NULL,

  -- surface ids involved (optional but useful for debugging)
  surface_a_id BIGINT,
  surface_b_id BIGINT,

  -- shared geometry/area
  shared_area DOUBLE PRECISION,
  shared_area_unit VARCHAR CHECK (shared_area_unit IN ('sqm')) DEFAULT 'sqm',

  -- optional: store intersection geometry for QA (can be heavy)
shared_geom GEOMETRY
);

-- Ensure pair uniqueness at building-level
CREATE UNIQUE INDEX IF NOT EXISTS ux_{lod_schema}_party_wall_pair
  ON {city2tabula_schema}.{lod_schema}_party_wall (lod, building_a_id, building_b_id, surface_a_id, surface_b_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_party_wall_building_a_idx
  ON {city2tabula_schema}.{lod_schema}_party_wall (building_a_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_party_wall_building_b_idx
  ON {city2tabula_schema}.{lod_schema}_party_wall (building_b_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_party_wall_geom_idx
  ON {city2tabula_schema}.{lod_schema}_party_wall USING GIST (shared_geom);



-- ============================================================
-- 6) Building feature table (unchanged, but keep your indexes)
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building_feature (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  building_feature_id BIGINT UNIQUE,
  tabula_variant_code_id INTEGER,
  tabula_variant_code VARCHAR,
  construction_year INTEGER,
  comment TEXT,
  heating_demand DOUBLE PRECISION,
  heating_demand_unit VARCHAR(20) CHECK (heating_demand_unit IN ('kWh/m2a')),
  footprint_area DOUBLE PRECISION,
  footprint_complexity INTEGER CHECK (footprint_complexity IN (0, 1, 2)),
  roof_complexity INTEGER CHECK (roof_complexity IN (0, 1, 2)),
  has_attached_neighbour BOOLEAN,
  attached_neighbour_id BIGINT[],
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
  building_footprint_geom GEOMETRY(MultiPolygonZ, {srid})
);

CREATE INDEX IF NOT EXISTS {lod_schema}_building_centroid_geometry_idx
  ON {city2tabula_schema}.{lod_schema}_building_feature USING GIST (building_centroid_geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_building_footprint_geometry_idx
  ON {city2tabula_schema}.{lod_schema}_building_feature USING GIST (building_footprint_geom);



-- ============================================================
-- 7) Candidate table indexes (fix the ones you had)
-- ============================================================
CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature USING GIST (geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_building_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature (building_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature (surface_feature_id);

