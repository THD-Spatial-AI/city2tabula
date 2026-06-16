DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lod INT NOT NULL,
    building_feature_id BIGINT NOT NULL,
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
    surface_feature_id INTEGER NOT NULL,
    building_object_id VARCHAR(100),
    surface_object_id  VARCHAR(100),
    objectclass_id INTEGER,
    classname TEXT,
    coord_dim INT,
    has_z BOOLEAN,
    geom geometry(POLYGONZ, {srid})
);

DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_child_feature_surface CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_child_feature_surface(
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  building_feature_id INTEGER,
  surface_feature_id INTEGER,
  building_object_id VARCHAR(100),
  surface_object_id  VARCHAR(100),
  objectclass_id INTEGER,
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
  is_party_wall BOOLEAN DEFAULT FALSE,
  neighbour_building_id INTEGER,
  child_row_id UUID,
  attribute_calc_status VARCHAR,
  geom geometry(POLYGONZ, {srid})
);

DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building_feature CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building_feature (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- object_id: the stable objectid from the source 3D city model (lod2.feature.objectid).
  -- Constant across re-imports; use this to identify the same physical building.
  object_id VARCHAR(100) UNIQUE,
  -- building_feature_id: session-local integer from lod2.feature.id.
  -- Changes on every re-import. Used only as a fast join key inside this pipeline.
  building_feature_id INTEGER UNIQUE,
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
  building_footprint_geom GEOMETRY(MultiPolygonZ, {srid})
);

-- Create indexes after tables are created
CREATE INDEX IF NOT EXISTS {lod_schema}_child_geometry_idx ON {city2tabula_schema}.{lod_schema}_child_feature USING GIST (geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_geometry_idx ON {city2tabula_schema}.{lod_schema}_child_feature_surface USING GIST (geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_building_centroid_geometry_idx ON {city2tabula_schema}.{lod_schema}_building_feature USING GIST (building_centroid_geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_building_footprint_geometry_idx ON {city2tabula_schema}.{lod_schema}_building_feature USING GIST (building_footprint_geom);
CREATE INDEX IF NOT EXISTS {lod_schema}_child_building_feature_id_idx ON {city2tabula_schema}.{lod_schema}_child_feature (id);
CREATE INDEX IF NOT EXISTS {lod_schema}_child_surface_building_feature_id_idx ON {city2tabula_schema}.{lod_schema}_child_feature_surface (building_feature_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_child_surface_feature_id_idx ON {city2tabula_schema}.{lod_schema}_child_feature_surface (surface_feature_id);
CREATE INDEX IF NOT EXISTS {lod_schema}_child_feature_surface_party_wall_idx ON {city2tabula_schema}.{lod_schema}_child_feature_surface (building_feature_id) WHERE is_party_wall = TRUE;

-- Neighbour pairs table (populated by neighbour detection scripts)
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_building_neighbours CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_building_neighbours (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    building_id_a INTEGER NOT NULL,
    building_id_b INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS {lod_schema}_building_neighbours_a_idx ON {city2tabula_schema}.{lod_schema}_building_neighbours (building_id_a);
CREATE INDEX IF NOT EXISTS {lod_schema}_building_neighbours_b_idx ON {city2tabula_schema}.{lod_schema}_building_neighbours (building_id_b);

-- Stable resolved building -> surface mapping using source objectids.
-- Populated by script 08 after all surface calculations and party-wall resolution
-- are complete. Survives re-imports: objectids are baked into the CityGML source
-- and never change, so this table can be used to skip the ST_3DIntersects resolve
-- step after a partial re-import of the same data.
--
-- One row per surface. building_object_id repeats; surface_object_id is unique
-- (each surface belongs to exactly one building after party-wall resolution).
DROP TABLE IF EXISTS {city2tabula_schema}.{lod_schema}_surface_link CASCADE;
CREATE TABLE {city2tabula_schema}.{lod_schema}_surface_link (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    building_object_id VARCHAR(100) NOT NULL,
    surface_object_id  VARCHAR(100) NOT NULL,
    surface_type    VARCHAR(50),   -- RoofSurface | WallSurface | GroundSurface
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    UNIQUE (building_object_id, surface_object_id)
);

-- Fast lookup: all surfaces for a given building
CREATE INDEX IF NOT EXISTS {lod_schema}_surface_link_building_idx
    ON {city2tabula_schema}.{lod_schema}_surface_link (building_object_id);

-- Enforce: each surface belongs to exactly one building
CREATE UNIQUE INDEX IF NOT EXISTS {lod_schema}_surface_link_surface_idx
    ON {city2tabula_schema}.{lod_schema}_surface_link (surface_object_id);