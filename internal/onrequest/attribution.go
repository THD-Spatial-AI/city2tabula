package onrequest

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// DatasetCredit is one source dataset's attribution, returned next to the data
// derived from it.
type DatasetCredit struct {
	DatasetID  string `json:"dataset_id"`
	Provider   string `json:"provider"`
	Dataset    string `json:"dataset"`
	Licence    string `json:"licence"`
	LicenceURL string `json:"licence_url"`
	Credit     string `json:"credit"`
	CreditURL  string `json:"credit_url,omitempty"`
	TermsURL   string `json:"terms_url,omitempty"`
	Changes    string `json:"changes"`
}

// AttributionStatus is a dataset's credit with the result of its last URL
// check. The check fields are null until a check has run.
type AttributionStatus struct {
	Country string `json:"country"`
	DatasetCredit
	URLCheckedAt     *time.Time        `json:"url_checked_at"`
	URLCheckOK       *bool             `json:"url_check_ok"`
	URLCheckFailures map[string]string `json:"url_check_failures"`
}

const creditColumns = `dataset_id, provider, dataset, licence, licence_url, credit,
	COALESCE(credit_url, ''), COALESCE(terms_url, ''), changes`

// BuildingDatasetIDs returns the distinct datasets behind buildings, plus the
// TABULA dataset when any building carries a TABULA type.
func BuildingDatasetIDs(buildings []Building) []string {
	var ids []string
	for _, b := range buildings {
		ids = append(ids, b.DatasetID)
		if b.TabulaVariantCode != nil {
			ids = append(ids, attribution.TabulaDatasetID)
		}
	}
	return distinct(ids)
}

// GeometryDatasetIDs returns the distinct datasets behind geometries.
func GeometryDatasetIDs(geometries []BuildingGeometry) []string {
	ids := make([]string, 0, len(geometries))
	for _, g := range geometries {
		ids = append(ids, g.DatasetID)
	}
	return distinct(ids)
}

func distinct(ids []string) []string {
	slices.Sort(ids)
	return slices.Compact(ids)
}

// CreditsFor returns the credit of every dataset in datasetIDs, as an empty
// slice for none. A dataset without a dataset_attribution row is an error: data
// is never returned without its credit.
func CreditsFor(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, datasetIDs []string) ([]DatasetCredit, error) {
	credits := []DatasetCredit{}
	if len(datasetIDs) == 0 {
		return credits, nil
	}
	q := fmt.Sprintf(`SELECT %s FROM %s.dataset_attribution WHERE dataset_id = ANY($1) ORDER BY dataset_id`,
		creditColumns, pgx.Identifier{cfg.DB.Schemas.City2Tabula}.Sanitize())
	rows, err := pool.Query(ctx, q, datasetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query attribution for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	for rows.Next() {
		var c DatasetCredit
		if err := rows.Scan(&c.DatasetID, &c.Provider, &c.Dataset, &c.Licence, &c.LicenceURL,
			&c.Credit, &c.CreditURL, &c.TermsURL, &c.Changes); err != nil {
			return nil, fmt.Errorf("failed to scan attribution row: %w", err)
		}
		credits = append(credits, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attribution rows: %w", err)
	}
	if len(credits) != len(datasetIDs) {
		return nil, fmt.Errorf("%s has no dataset_attribution row for some of %v; run -sync-attribution", cfg.DB.Name, datasetIDs)
	}
	return credits, nil
}

// AttributionStatuses returns every dataset_attribution row of cfg's database
// with its last URL check.
func AttributionStatuses(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) ([]AttributionStatus, error) {
	q := fmt.Sprintf(`SELECT %s, url_checked_at, url_check_ok, url_check_failures
		FROM %s.dataset_attribution ORDER BY dataset_id`,
		creditColumns, pgx.Identifier{cfg.DB.Schemas.City2Tabula}.Sanitize())
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query attribution status for %s: %w", cfg.Country, err)
	}
	defer rows.Close()
	var out []AttributionStatus
	for rows.Next() {
		s := AttributionStatus{Country: cfg.Country}
		c := &s.DatasetCredit
		if err := rows.Scan(&c.DatasetID, &c.Provider, &c.Dataset, &c.Licence, &c.LicenceURL,
			&c.Credit, &c.CreditURL, &c.TermsURL, &c.Changes,
			&s.URLCheckedAt, &s.URLCheckOK, &s.URLCheckFailures); err != nil {
			return nil, fmt.Errorf("failed to scan attribution status row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attribution status rows: %w", err)
	}
	return out, nil
}
