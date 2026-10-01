//go:build integration

package process_test

import (
	"context"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// seedLOD3Building sets up fresh City2TABULA tables and inserts one extracted
// LOD3 building with one wall surface, with nothing in the LOD2 tables. A LOD3
// dataset (e.g. Prague's CityGML) extracts into exactly this shape.
func seedLOD3Building(t *testing.T, ctx context.Context, objectID string) *config.Config {
	t.Helper()
	resetSchemas(t)
	cfg := pipelineConfig(pipelineTestCase{country: "germany", srid: "25832"})
	if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}

	for _, stmt := range []string{
		`INSERT INTO city2tabula.lod3_building
			(object_id, country_code, building_feature_id, footprint_area,
			 tabula_variant_code, building_footprint_geom)
		VALUES ($1, 'DE', 9001, 100.0, 'DE.N.SFH.01.Gen.ReEx.001.001',
			ST_Multi(ST_GeomFromText('POLYGON Z((500000 5400000 0, 500010 5400000 0, 500010 5400010 0, 500000 5400010 0, 500000 5400000 0))', 25832)))`,
		`INSERT INTO city2tabula.lod3_surface
			(building_object_id, surface_object_id, surface_type, surface_area, geom)
		VALUES ($1, 'lod3-wall', 'WallSurface', 30.0,
			ST_GeomFromText('POLYGON Z((500000 5400000 0, 500010 5400000 0, 500010 5400000 3, 500000 5400000 3, 500000 5400000 0))', 25832))`,
	} {
		if _, err := testPool.Exec(ctx, stmt, objectID); err != nil {
			t.Fatalf("seed LOD3 building %s: %v", objectID, err)
		}
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM city2tabula.building_link WHERE object_id = $1`,
			`DELETE FROM city2tabula.lod3_surface WHERE building_object_id = $1`,
			`DELETE FROM city2tabula.lod3_building WHERE object_id = $1`,
		} {
			_, _ = testPool.Exec(ctx, q, objectID)
		}
	})
	return cfg
}

// TestServer_ReadsLOD3Buildings pins that the on-request queries serve a
// database whose buildings were extracted at LOD3 only.
func TestServer_ReadsLOD3Buildings(t *testing.T) {
	ctx := context.Background()
	cfg := seedLOD3Building(t, ctx, "lod3-served")

	var bbox onrequest.Bbox
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope(499990, 5399990, 500020, 5400020, 25832), 4326) AS e) s`,
	).Scan(&bbox.Xmin, &bbox.Ymin, &bbox.Xmax, &bbox.Ymax); err != nil {
		t.Fatalf("build bbox: %v", err)
	}

	buildings, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bbox)
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	if len(buildings) != 1 || buildings[0].ObjectID != "lod3-served" || len(buildings[0].Surfaces) != 1 {
		t.Fatalf("BuildingsByBBox: want 1 LOD3 building with 1 surface, got %+v", buildings)
	}

	geometry, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{"lod3-served"}, true)
	if err != nil {
		t.Fatalf("BuildingGeometryByObjectIDs: %v", err)
	}
	if len(geometry) != 1 || geometry[0].FootprintGeoJSON == nil || len(geometry[0].Surfaces) != 1 {
		t.Fatalf("BuildingGeometryByObjectIDs: want 1 LOD3 footprint with 1 surface, got %+v", geometry)
	}
}

// TestServer_ServesOlderReleaseSchema pins that the on-request queries still
// serve a database built by an earlier release, where area_below_precision was
// added to lod2_surface alone and neither surface table has length or width.
func TestServer_ServesOlderReleaseSchema(t *testing.T) {
	ctx := context.Background()
	cfg := seedLOD3Building(t, ctx, "older-schema")
	for _, stmt := range []string{
		`ALTER TABLE city2tabula.lod2_surface DROP COLUMN IF EXISTS length, DROP COLUMN IF EXISTS width`,
		`ALTER TABLE city2tabula.lod3_surface DROP COLUMN IF EXISTS length, DROP COLUMN IF EXISTS width, DROP COLUMN area_below_precision`,
	} {
		if _, err := testPool.Exec(ctx, stmt); err != nil {
			t.Fatalf("reshape surface tables: %v", err)
		}
	}
	t.Cleanup(func() {
		if err := db.RunCity2TabulaDBSetup(cfg, testPool); err != nil {
			t.Errorf("restore current schema: %v", err)
		}
	})

	geometry, err := onrequest.BuildingGeometryByObjectIDs(ctx, testPool, cfg, []string{"older-schema"}, true)
	if err != nil {
		t.Fatalf("BuildingGeometryByObjectIDs: %v", err)
	}
	if len(geometry) != 1 || len(geometry[0].Surfaces) != 1 {
		t.Fatalf("BuildingGeometryByObjectIDs: want 1 building with 1 surface, got %+v", geometry)
	}

	var bbox onrequest.Bbox
	if err := testPool.QueryRow(ctx, `
		SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		FROM (SELECT ST_Transform(ST_MakeEnvelope(499990, 5399990, 500020, 5400020, 25832), 4326) AS e) s`,
	).Scan(&bbox.Xmin, &bbox.Ymin, &bbox.Xmax, &bbox.Ymax); err != nil {
		t.Fatalf("build bbox: %v", err)
	}
	buildings, err := onrequest.BuildingsByBBox(ctx, testPool, cfg, bbox)
	if err != nil {
		t.Fatalf("BuildingsByBBox: %v", err)
	}
	if len(buildings) != 1 || len(buildings[0].Surfaces) != 1 {
		t.Fatalf("BuildingsByBBox: want 1 building with 1 surface, got %+v", buildings)
	}
	if abp := buildings[0].Surfaces[0].AreaBelowPrecision; abp == nil || *abp {
		t.Errorf("area_below_precision for a 30 m2 wall: want false, got %v", abp)
	}
}

// TestRunPyLovoLinkBuild_LinksLOD3Buildings pins that -link-pylovo matches
// LOD3 buildings, and that the linked building is then served by OSM id.
func TestRunPyLovoLinkBuild_LinksLOD3Buildings(t *testing.T) {
	ctx := context.Background()
	cfg := seedLOD3Building(t, ctx, "lod3-linked")
	cfg.DB.Schemas.Pylvo = "public"
	cfg.City2Tabula.LinkGridSize = 1000

	seedPylovoTestTables(t, ctx)
	if _, err := testPool.Exec(ctx, `
		INSERT INTO public.res (osm_id, country_code, geom)
		SELECT 'RES-LOD3', country_code, ST_Multi(ST_Force2D(ST_Transform(building_footprint_geom, 3035)))
		FROM city2tabula.lod3_building WHERE object_id = 'lod3-linked'`,
	); err != nil {
		t.Fatalf("seed pylovo.res: %v", err)
	}

	if err := process.RunPyLovoLinkBuild(cfg, testPool); err != nil {
		t.Fatalf("RunPyLovoLinkBuild: %v", err)
	}

	link := readBuildingLink(t, ctx, "lod3-linked")
	if link.matchType != 1 || link.osmID == nil || *link.osmID != "RES-LOD3" {
		t.Fatalf("want lod3-linked matched to RES-LOD3, got match_type=%d osm_id=%v", link.matchType, link.osmID)
	}

	buildings, err := onrequest.BuildingsByOSMIDs(ctx, testPool, cfg, []string{"RES-LOD3"})
	if err != nil {
		t.Fatalf("BuildingsByOSMIDs: %v", err)
	}
	if len(buildings) != 1 || buildings[0].ObjectID != "lod3-linked" {
		t.Fatalf("BuildingsByOSMIDs: want lod3-linked, got %+v", buildings)
	}
}
