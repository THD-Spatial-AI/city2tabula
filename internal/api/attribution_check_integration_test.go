//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/api/handler"
	"github.com/thd-spatial-ai/city2tabula/internal/api/router"
	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// getJSON requests url, requires status, and decodes the body into out unless
// out is nil.
func getJSON(t *testing.T, url string, status int, out any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		t.Fatalf("GET %s: status %d, want %d", url, resp.StatusCode, status)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}

// The server checks the attribution URLs of every country database it can
// serve, records each row's result there, skips countries without a database
// rather than creating one, and lists the results at GET /api/v1/attributions.
func TestServer_CheckAttributionURLs_RecordsPerCountryDatabase(t *testing.T) {
	ctx := context.Background()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Chdir(filepath.Join(wd, "..", ".."))

	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/missing", http.NotFound)
	urls := httptest.NewServer(mux)
	defer urls.Close()

	host, port := testutil.StartPostGISAddr(t)
	base := baseServerConfig(host, port, "attrcheck")
	cfg, err := config.RegionConfig(base, "germany")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	pool, err := db.ConnectPool(&cfg)
	if err != nil {
		t.Fatalf("ConnectPool: %v", err)
	}
	t.Cleanup(func() { db.ClosePool(pool) })
	if err := db.RunCity2TabulaDBSetup(&cfg, pool); err != nil {
		t.Fatalf("RunCity2TabulaDBSetup: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO city2tabula.dataset_attribution
			(dataset_id, provider, dataset, licence, licence_url, credit, credit_url, changes, source_path)
		VALUES ('de-good-lod2', 'P', 'D', 'CC-BY-4.0', $1, 'C', NULL, 'X', 'data'),
		       ('de-bad-lod2',  'P', 'D', 'CC-BY-4.0', $1, 'C', $2,   'X', 'data')`,
		urls.URL+"/ok", urls.URL+"/missing"); err != nil {
		t.Fatalf("seed dataset_attribution: %v", err)
	}

	srv := server.New(base)
	srv.CheckAttributionURLs(ctx, urls.Client())

	for id, want := range map[string]bool{"de-good-lod2": true, "de-bad-lod2": false} {
		var ok *bool
		if err := pool.QueryRow(ctx,
			`SELECT url_check_ok FROM city2tabula.dataset_attribution WHERE dataset_id = $1`, id,
		).Scan(&ok); err != nil {
			t.Fatalf("read url_check_ok of %s: %v", id, err)
		}
		if ok == nil || *ok != want {
			t.Errorf("%s: url_check_ok = %v, want %v", id, ok, want)
		}
	}

	// GET /api/v1/attributions lists both rows with their check result, for
	// every country and for germany alone; a country without a database lists
	// nothing and an unknown one is refused.
	api := httptest.NewServer(router.New(handler.New(srv)))
	defer api.Close()
	for _, query := range []string{"", "?country=germany"} {
		var body struct {
			Attributions []struct {
				Country          string            `json:"country"`
				DatasetID        string            `json:"dataset_id"`
				Credit           string            `json:"credit"`
				URLCheckOK       *bool             `json:"url_check_ok"`
				URLCheckedAt     *string           `json:"url_checked_at"`
				URLCheckFailures map[string]string `json:"url_check_failures"`
			} `json:"attributions"`
		}
		getJSON(t, api.URL+"/api/v1/attributions"+query, http.StatusOK, &body)
		got := body.Attributions
		if len(got) != 2 || got[0].DatasetID != "de-bad-lod2" || got[1].DatasetID != "de-good-lod2" {
			t.Fatalf("attributions%s = %+v, want de-bad-lod2 and de-good-lod2", query, got)
		}
		bad, good := got[0], got[1]
		if bad.Country != "germany" || bad.URLCheckOK == nil || *bad.URLCheckOK || bad.URLCheckFailures[urls.URL+"/missing"] == "" || bad.URLCheckedAt == nil {
			t.Errorf("attributions%s de-bad-lod2 = %+v, want germany, failed, with the missing URL and a check time", query, bad)
		}
		if good.URLCheckOK == nil || !*good.URLCheckOK || good.URLCheckFailures != nil || good.Credit == "" {
			t.Errorf("attributions%s de-good-lod2 = %+v, want passed, no failures, a credit", query, good)
		}
	}
	var empty struct {
		Attributions []any `json:"attributions"`
	}
	getJSON(t, api.URL+"/api/v1/attributions?country=netherlands", http.StatusOK, &empty)
	if empty.Attributions == nil || len(empty.Attributions) != 0 {
		t.Errorf("attributions for a country without a database = %v, want []", empty.Attributions)
	}
	getJSON(t, api.URL+"/api/v1/attributions?country=atlantis", http.StatusBadRequest, nil)

	// Data is never served without its credit: a dataset with no row is an error.
	if _, err := onrequest.CreditsFor(ctx, pool, &cfg, []string{"de-good-lod2", "tabula-episcope"}); err == nil ||
		!strings.Contains(err.Error(), "-sync-attribution") {
		t.Errorf("CreditsFor with a dataset lacking a row: %v, want an error pointing at -sync-attribution", err)
	}

	nl, err := config.RegionConfig(base, "netherlands")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	if exists, err := db.DatabaseExists(&nl); err != nil || exists {
		t.Errorf("DatabaseExists(%s) = %v, %v; the check must not create a database", nl.DB.Name, exists, err)
	}
}
