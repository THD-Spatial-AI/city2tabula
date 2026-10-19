package process

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// linkKeyIndex is the unique index of a building_link table that holds one row per
// OSM way (sql/schema/main/02_create_link_tables.sql). A table without it predates
// match_type 4 and allows one row per 3D building only.
const linkKeyIndex = "building_link_object_osm_country_key"

// ensureLinkTableShape brings a building_link table created by an earlier release
// to the current shape, so the link step can store every OSM way of a building:
// match_type 4 allowed, unique per (object_id, osm_id, country_code). It is a no-op
// on a current table. The advisory lock serialises runs that link disjoint regions
// in parallel, which would otherwise race on the same DDL.
func ensureLinkTableShape(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	schema := cfg.DB.Schemas.City2Tabula
	table := pgx.Identifier{schema, "building_link"}.Sanitize()
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, table); err != nil {
			return err
		}
		var current bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+linkKeyIndex).Scan(&current); err != nil {
			return err
		}
		if current {
			return nil
		}
		for _, stmt := range []string{
			`ALTER TABLE ` + table + ` DROP CONSTRAINT IF EXISTS building_link_object_id_country_code_key`,
			`ALTER TABLE ` + table + ` DROP CONSTRAINT IF EXISTS building_link_match_type_check`,
			`ALTER TABLE ` + table + ` ADD CONSTRAINT building_link_match_type_check CHECK (match_type IN (1, 2, 3, 4))`,
			`CREATE UNIQUE INDEX ` + linkKeyIndex + ` ON ` + table + ` (object_id, osm_id, country_code) NULLS NOT DISTINCT`,
		} {
			if _, err := tx.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w", stmt, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update %s to one row per OSM way: %w", table, err)
	}
	return nil
}
