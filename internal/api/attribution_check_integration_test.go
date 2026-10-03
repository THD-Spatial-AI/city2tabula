//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// The server checks the attribution URLs of every country database it can
// serve, records each row's result there, and skips countries without a
// database rather than creating one.
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

	server.New(base).CheckAttributionURLs(ctx, urls.Client())

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

	nl, err := config.RegionConfig(base, "netherlands")
	if err != nil {
		t.Fatalf("RegionConfig: %v", err)
	}
	if exists, err := db.DatabaseExists(&nl); err != nil || exists {
		t.Errorf("DatabaseExists(%s) = %v, %v; the check must not create a database", nl.DB.Name, exists, err)
	}
}
