// Package onrequest orchestrates a single on-request City2TABULA run for one
// country/bbox: import (creating the country's database from scratch on first
// use, or importing into it incrementally thereafter), feature extraction, and
// PyLovo linking. This is the same sequence cmd/c2t/main.go runs via three
// separate CLI flags, collapsed into one call for the HTTP server (internal/api),
// which triggers it per request instead of per invocation.
package onrequest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/process"
)

// Bbox is a WGS84 (EPSG:4326) lon/lat bounding box — the CRS a web frontend's
// drawn area naturally comes in. citydb-tool and the coverage query both handle
// reprojecting to the target country's native CRS themselves.
type Bbox struct {
	Xmin, Ymin, Xmax, Ymax float64
}

// String formats b for citydb-tool's --bbox flag (xmin,ymin,xmax,ymax,srid).
func (b Bbox) String() string {
	return fmt.Sprintf("%g,%g,%g,%g,4326", b.Xmin, b.Ymin, b.Xmax, b.Ymax)
}

// RunForRegion imports 3D data for cfg's country, scoped to bbox, then extracts
// features and links against PyLovo. cfg must come from config.RegionConfig —
// Country, DB.Name, CityDB.SRID/SRSName, and Data.Lod2/Lod3 all need to already
// be resolved for the target country. bboxMode is citydb-tool's --bbox-mode
// (intersects, contains, or on_tile).
func RunForRegion(cfg *config.Config, bbox Bbox, bboxMode string) error {
	existed, err := db.DatabaseExists(cfg)
	if err != nil {
		return fmt.Errorf("failed to check whether database %s exists: %w", cfg.DB.Name, err)
	}

	pool, err := db.ConnectPool(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database %s: %w", cfg.DB.Name, err)
	}
	defer db.ClosePool(pool)

	if existed {
		if err := db.ImportAllData(cfg, pool, bbox.String(), bboxMode); err != nil {
			return fmt.Errorf("failed to import data for %s: %w", cfg.Country, err)
		}
	} else {
		if err := db.CreateCompleteDatabase(cfg, pool, bbox.String(), bboxMode); err != nil {
			return fmt.Errorf("failed to create database for %s: %w", cfg.Country, err)
		}
	}

	if err := process.RunFeatureExtraction(cfg, pool); err != nil {
		return fmt.Errorf("failed to extract features for %s: %w", cfg.Country, err)
	}

	if err := process.RunPyLovoLinkBuild(cfg, pool); err != nil {
		return fmt.Errorf("failed to link PyLovo buildings for %s: %w", cfg.Country, err)
	}

	return nil
}

// CountBuildingLink returns how many building_link rows exist for cfg's country
// within bbox. A zero count after RunForRegion succeeds means citydb-tool found no
// source data for that area — the caller (internal/api) uses this to distinguish
// "ran fine, nothing there" from "ran fine, buildings found".
func CountBuildingLink(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, bbox Bbox) (int, error) {
	q := fmt.Sprintf(`
		SELECT count(*) FROM %s.building_link
		WHERE country_code = $1
		  AND ST_Intersects(geom, ST_Transform(ST_MakeEnvelope($2,$3,$4,$5,4326), $6::int))`,
		cfg.DB.Schemas.City2Tabula,
	)

	var count int
	if err := pool.QueryRow(ctx, q,
		cfg.CountryCode, bbox.Xmin, bbox.Ymin, bbox.Xmax, bbox.Ymax, cfg.CityDB.SRID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count building_link rows for %s: %w", cfg.Country, err)
	}
	return count, nil
}
