package process

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// checkBuildingDatasets fails when a Building feature in schema has no lineage,
// or a lineage with no dataset_attribution row. Script 04 copies the lineage
// into the NOT NULL dataset_id foreign key, so either case would fail that
// script for a whole batch without saying which dataset is missing.
func checkBuildingDatasets(pool *pgxpool.Pool, cfg *config.Config, schema string) error {
	q := fmt.Sprintf(`
		SELECT COUNT(*) FILTER (WHERE f.lineage IS NULL),
		       COALESCE(MIN(f.objectid) FILTER (WHERE f.lineage IS NULL), ''),
		       COUNT(*) FILTER (WHERE f.lineage IS NOT NULL AND a.dataset_id IS NULL),
		       COALESCE(MIN(f.lineage) FILTER (WHERE f.lineage IS NOT NULL AND a.dataset_id IS NULL), '')
		FROM %s.feature f
		LEFT JOIN %s.dataset_attribution a ON a.dataset_id = f.lineage
		WHERE f.objectclass_id = 901`, schema, cfg.DB.Schemas.City2Tabula)

	var noLineage, noRow int
	var exampleObject, exampleDataset string
	if err := pool.QueryRow(context.Background(), q).Scan(&noLineage, &exampleObject, &noRow, &exampleDataset); err != nil {
		return fmt.Errorf("check dataset lineage of buildings in %s: %w", schema, err)
	}

	var errs []error
	if noLineage > 0 {
		errs = append(errs, fmt.Errorf(
			"%d buildings in %s have no dataset lineage (e.g. object id %s); import them again from a dataset folder holding an attribution.json",
			noLineage, schema, exampleObject))
	}
	if noRow > 0 {
		errs = append(errs, fmt.Errorf(
			"%d buildings in %s have a lineage with no dataset_attribution row (e.g. %q); run -sync-attribution",
			noRow, schema, exampleDataset))
	}
	return errors.Join(errs...)
}
