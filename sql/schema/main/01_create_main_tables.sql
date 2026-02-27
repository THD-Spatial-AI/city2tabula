-- ============================================================
-- 1) Candidate mapping table (dirty / many-to-many)
-- ============================================================
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lod INT NOT NULL,
    building_feature_id BIGINT NOT NULL,
    building_objectid TEXT,
    surface_feature_id BIGINT NOT NULL,
    surface_objectid TEXT,
    objectclass_id INT,
    classname  VARCHAR(255),
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
    building_objectid TEXT,
    surface_feature_id BIGINT NOT NULL,
    surface_objectid TEXT,
    objectclass_id INT,
    classname  VARCHAR(255),
    coord_dim INT,
    has_z BOOLEAN,
    geom geometry(POLYGONZ, {srid})
);

-- Indexes on geom_dump for surface resolvers (ground/wall/roof read from here)
CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_geom_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump USING GIST (geom);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_building_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (building_feature_id);

CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_geom_dump_surface_idx
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (surface_feature_id);

CREATE INDEX IF NOT EXISTS idx_{lod_schema}_child_geom_dump_by_class_building
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (classname, building_feature_id);

CREATE INDEX IF NOT EXISTS idx_{lod_schema}_child_geom_dump_by_class_surface
  ON {city2tabula_schema}.{lod_schema}_child_feature_geom_dump (classname, surface_feature_id);

-- Evidence table (unchanged)
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_surface CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_surface (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  building_feature_id BIGINT,
  building_objectid TEXT,
  surface_feature_id  BIGINT,
  surface_objectid TEXT,

  objectclass_id INT,
  classname      VARCHAR(255),

  height DOUBLE PRECISION,
  height_unit VARCHAR CHECK (height_unit IN ('m')),

  surface_area DOUBLE PRECISION,
  surface_area_unit VARCHAR CHECK (surface_area_unit IN ('sqm')),

  tilt DOUBLE PRECISION,
  tilt_unit VARCHAR CHECK (tilt_unit IN ('degrees')),

  azimuth DOUBLE PRECISION,
  azimuth_unit VARCHAR CHECK (azimuth_unit IN ('degrees')),

  is_valid  BOOLEAN,
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


-- Resolved mapping table
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_resolved CASCADE;

CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_resolved (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  lod INT NOT NULL,

  -- resolved ownership
  building_feature_id BIGINT NOT NULL,
  building_objectid TEXT,
  surface_feature_id  BIGINT NOT NULL,
  surface_objectid TEXT,

  objectclass_id INT,
  classname      VARCHAR(255),

  -- resolution metadata
  score DOUBLE PRECISION,
  scoring_method VARCHAR(64),
  resolved_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  -- party wall support (mainly for WallSurface)
  is_party_wall BOOLEAN NOT NULL DEFAULT FALSE,
  party_with_building_id BIGINT,

  -- original surface geometry (keep generic MultiPolygonZ)
  geom geometry(MultiPolygonZ, {srid})
);

-- ------------------------------------------------------------
-- Constraints / Keys
-- ------------------------------------------------------------

-- One row per surface per LoD (this is the ON CONFLICT target)
CREATE UNIQUE INDEX ux_{lod_schema}_resolved_lod_surface
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (lod, surface_feature_id);

-- ------------------------------------------------------------
-- Lookup indexes used by the pipeline
-- ------------------------------------------------------------

-- Common lookup: (lod, classname, building_feature_id)
CREATE INDEX idx_{lod_schema}_resolved_by_building_class
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (lod, classname, building_feature_id);

-- Common lookup: (lod, classname, surface_feature_id)
CREATE INDEX idx_{lod_schema}_resolved_by_surface_class
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (lod, classname, surface_feature_id);

-- Fast GroundSurface lookup for resolvers (small partial index)
CREATE INDEX idx_{lod_schema}_resolved_ground_by_building
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (lod, building_feature_id)
  WHERE classname = 'GroundSurface';

-- Party-wall queries
CREATE INDEX idx_{lod_schema}_resolved_party
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved (is_party_wall, party_with_building_id);

-- Optional: spatial queries on resolved output (not needed for resolving itself)
CREATE INDEX gix_{lod_schema}_resolved_geom
  ON {city2tabula_schema}.{lod_schema}_child_feature_resolved USING GIST (geom);

-- ============================================================
-- Core indexes on source tables (needed for wall/roof resolving)
-- ============================================================

-- Stage-1 mapping: candidate surfaces per building
CREATE INDEX IF NOT EXISTS idx_{lod_schema}_child_feat_by_lod_class_building
  ON {city2tabula_schema}.{lod_schema}_child_feature (lod, classname, building_feature_id);

-- Stage-1 mapping: reverse lookup / claims per surface
CREATE INDEX IF NOT EXISTS idx_{lod_schema}_child_feat_by_lod_class_surface
  ON {city2tabula_schema}.{lod_schema}_child_feature (lod, classname, surface_feature_id);

-- Stage-3 surfaces: geometry lookup by (classname, surface_feature_id)
CREATE INDEX IF NOT EXISTS idx_{lod_schema}_child_surface_by_class_surface
  ON {city2tabula_schema}.{lod_schema}_child_feature_surface (classname, surface_feature_id);

-- ------------------------------------------------------------
-- Refresh planner stats (recommended after creating indexes)
-- ------------------------------------------------------------
ANALYSE {city2tabula_schema}.{lod_schema}_child_feature_resolved;
ANALYSE {city2tabula_schema}.{lod_schema}_child_feature;
ANALYSE {city2tabula_schema}.{lod_schema}_child_feature_surface;

DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building_feature (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  building_feature_id BIGINT UNIQUE,
  building_objectid TEXT,
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
  building_footprint_geom GEOMETRY(MultiPolygon, {srid})
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

