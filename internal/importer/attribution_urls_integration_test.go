//go:build integration

package importer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
)

// urlTestServer answers /ok and fails /missing, counting the /ok requests.
func urlTestServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var okHits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { okHits.Add(1) })
	mux.HandleFunc("/missing", http.NotFound)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &okHits
}

// pointDatasetsAt rewrites the three test datasets so every URL is on srv:
// region-a and TABULA answer, region-b's credit_url does not.
func pointDatasetsAt(t *testing.T, cfg *config.Config, srv *httptest.Server) {
	t.Helper()
	a := testAttribution("xx-region-a-lod2", "Credit A", srv.URL+"/ok")
	a.CreditURL = srv.URL + "/ok"
	writeAttributionFile(t, filepath.Join(cfg.Data.Lod2, "region-a"), a)
	b := testAttribution("xx-region-b-lod3", "Credit B", srv.URL+"/ok")
	b.CreditURL = srv.URL + "/missing"
	writeAttributionFile(t, filepath.Join(cfg.Data.Lod3, "region-b"), b)
	writeAttributionFile(t, cfg.Data.Tabula, testAttribution("tabula-episcope", "Credit T", srv.URL+"/ok"))
}

type urlCheckRow struct {
	checkedAt *time.Time
	ok        *bool
	failures  map[string]string
}

func readURLChecks(t *testing.T, ctx context.Context, schema string) map[string]urlCheckRow {
	t.Helper()
	rows, err := testPool.Query(ctx, `SELECT dataset_id, url_checked_at, url_check_ok, url_check_failures FROM `+schema+`.dataset_attribution`)
	if err != nil {
		t.Fatalf("query URL checks: %v", err)
	}
	defer rows.Close()
	got := map[string]urlCheckRow{}
	for rows.Next() {
		var id string
		var r urlCheckRow
		if err := rows.Scan(&id, &r.checkedAt, &r.ok, &r.failures); err != nil {
			t.Fatalf("scan URL checks: %v", err)
		}
		got[id] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate URL checks: %v", err)
	}
	return got
}

func TestCheckAttributionURLs_RecordsEachRowsResult(t *testing.T) {
	ctx := context.Background()
	cfg, _ := syncConfig(t, ctx)
	schema := cfg.DB.Schemas.City2Tabula
	srv, okHits := urlTestServer(t)
	pointDatasetsAt(t, cfg, srv)
	if _, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil {
		t.Fatalf("SyncAttribution: %v", err)
	}

	failures, err := importer.CheckAttributionURLs(ctx, testPool, schema, srv.Client())
	if err != nil {
		t.Fatalf("CheckAttributionURLs: %v", err)
	}

	if len(failures) != 1 || failures[0].DatasetID != "xx-region-b-lod3" ||
		failures[0].Field != "credit_url" || failures[0].URL != srv.URL+"/missing" {
		t.Fatalf("failures = %+v, want one: xx-region-b-lod3 credit_url %s/missing", failures, srv.URL)
	}
	// Five URL fields across three rows point at /ok; each distinct URL is
	// requested once.
	if n := okHits.Load(); n != 1 {
		t.Errorf("/ok requested %d times, want 1", n)
	}

	got := readURLChecks(t, ctx, schema)
	for _, id := range []string{"xx-region-a-lod2", "tabula-episcope"} {
		r := got[id]
		if r.checkedAt == nil || r.ok == nil || !*r.ok || r.failures != nil {
			t.Errorf("%s: checked_at %v, ok %v, failures %v; want a time, true and NULL", id, r.checkedAt, r.ok, r.failures)
		}
	}
	b := got["xx-region-b-lod3"]
	if b.checkedAt == nil || b.ok == nil || *b.ok || b.failures[srv.URL+"/missing"] == "" {
		t.Errorf("xx-region-b-lod3: checked_at %v, ok %v, failures %v; want a time, false and the missing URL", b.checkedAt, b.ok, b.failures)
	}
}

// A check result describes the URLs it checked, so editing a URL clears it,
// while editing anything else keeps it.
func TestSyncAttribution_ClearsURLCheckOnlyWhenAURLChanges(t *testing.T) {
	ctx := context.Background()
	cfg, _ := syncConfig(t, ctx)
	schema := cfg.DB.Schemas.City2Tabula
	srv, _ := urlTestServer(t)
	pointDatasetsAt(t, cfg, srv)
	if _, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil {
		t.Fatalf("SyncAttribution: %v", err)
	}
	if _, err := importer.CheckAttributionURLs(ctx, testPool, schema, srv.Client()); err != nil {
		t.Fatalf("CheckAttributionURLs: %v", err)
	}
	before := readURLChecks(t, ctx, schema)

	a := testAttribution("xx-region-a-lod2", "Corrected credit A", srv.URL+"/ok")
	a.CreditURL = srv.URL + "/ok"
	writeAttributionFile(t, filepath.Join(cfg.Data.Lod2, "region-a"), a)
	b := testAttribution("xx-region-b-lod3", "Credit B", srv.URL+"/ok")
	b.CreditURL = srv.URL + "/ok"
	writeAttributionFile(t, filepath.Join(cfg.Data.Lod3, "region-b"), b)
	if n, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil || n != 2 {
		t.Fatalf("second SyncAttribution = %d, %v; want 2, nil", n, err)
	}

	after := readURLChecks(t, ctx, schema)
	if ra := after["xx-region-a-lod2"]; ra.checkedAt == nil || !ra.checkedAt.Equal(*before["xx-region-a-lod2"].checkedAt) || ra.ok == nil || !*ra.ok {
		t.Errorf("credit text edit cleared or changed the check result: before %+v, after %+v", before["xx-region-a-lod2"], ra)
	}
	if rb := after["xx-region-b-lod3"]; rb.checkedAt != nil || rb.ok != nil || rb.failures != nil {
		t.Errorf("credit_url edit kept the old check result: %+v", rb)
	}
}
