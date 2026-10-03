package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
)

// URLFailure is one attribution URL that did not answer.
type URLFailure struct {
	DatasetID string
	Field     string
	URL       string
	Err       error
}

// datasetURLs is one dataset_attribution row's URLs, keyed by column name.
type datasetURLs struct {
	datasetID string
	urls      [][2]string
}

// CheckAttributionURLs checks every URL of every dataset_attribution row in
// schema, records each row's result, and returns the failures. A URL shared by
// several rows is requested once. A failure is recorded and reported, never
// acted on: the dataset stays served.
func CheckAttributionURLs(ctx context.Context, conn *pgxpool.Pool, schema string, client *http.Client) ([]URLFailure, error) {
	rows, err := readAttributionURLs(ctx, conn, schema)
	if err != nil {
		return nil, err
	}

	results := map[string]error{}
	var failures []URLFailure
	for _, row := range rows {
		rowFailures := map[string]string{}
		for _, field := range row.urls {
			name, url := field[0], field[1]
			checkErr, seen := results[url]
			if !seen {
				checkErr = attribution.CheckURL(ctx, client, url)
				results[url] = checkErr
			}
			if checkErr != nil {
				rowFailures[url] = checkErr.Error()
				failures = append(failures, URLFailure{DatasetID: row.datasetID, Field: name, URL: url, Err: checkErr})
			}
		}
		if err := recordURLCheck(ctx, conn, schema, row.datasetID, rowFailures); err != nil {
			return nil, err
		}
	}
	return failures, nil
}

// readAttributionURLs returns the non-empty URLs of every dataset_attribution row.
func readAttributionURLs(ctx context.Context, conn *pgxpool.Pool, schema string) ([]datasetURLs, error) {
	q := fmt.Sprintf(`SELECT dataset_id, licence_url, COALESCE(credit_url, ''), COALESCE(terms_url, '')
		FROM %s.dataset_attribution ORDER BY dataset_id`, pgx.Identifier{schema}.Sanitize())
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("read attribution URLs from %s: %w", schema, err)
	}
	defer rows.Close()

	var out []datasetURLs
	for rows.Next() {
		var id, licence, credit, terms string
		if err := rows.Scan(&id, &licence, &credit, &terms); err != nil {
			return nil, fmt.Errorf("scan attribution URLs from %s: %w", schema, err)
		}
		row := datasetURLs{datasetID: id}
		for _, f := range [][2]string{{"licence_url", licence}, {"credit_url", credit}, {"terms_url", terms}} {
			if f[1] != "" {
				row.urls = append(row.urls, f)
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// recordURLCheck stores one row's check result. url_check_failures is NULL when
// every URL answered.
func recordURLCheck(ctx context.Context, conn *pgxpool.Pool, schema, datasetID string, failures map[string]string) error {
	var failuresJSON []byte
	if len(failures) > 0 {
		var err error
		if failuresJSON, err = json.Marshal(failures); err != nil {
			return fmt.Errorf("encode URL failures of %s: %w", datasetID, err)
		}
	}
	q := fmt.Sprintf(`UPDATE %s.dataset_attribution
		SET url_checked_at = NOW(), url_check_ok = $2, url_check_failures = $3::jsonb
		WHERE dataset_id = $1`, pgx.Identifier{schema}.Sanitize())
	if _, err := conn.Exec(ctx, q, datasetID, len(failures) == 0, failuresJSON); err != nil {
		return fmt.Errorf("record URL check of %s: %w", datasetID, err)
	}
	return nil
}
