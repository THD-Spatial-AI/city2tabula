//go:build integration

package importer_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
)

// writeAttributionFolder creates dir with an attribution.json for datasetID
// carrying credit, and the gml subfolder a dataset folder keeps its files in.
func writeAttributionFolder(t *testing.T, dir, datasetID, credit string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "gml"), 0o755); err != nil {
		t.Fatalf("create dataset folder: %v", err)
	}
	data, err := json.Marshal(attribution.Attribution{
		SchemaVersion: attribution.SchemaVersion,
		DatasetID:     datasetID,
		Provider:      "Example Provider",
		Dataset:       "Example buildings",
		Licence:       "CC-BY-4.0",
		LicenceURL:    "https://creativecommons.org/licenses/by/4.0/",
		Credit:        credit,
		Changes:       "City2TABULA derives building attributes from the geometry.",
	})
	if err != nil {
		t.Fatalf("marshal attribution: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, attribution.FileName), data, 0o644); err != nil {
		t.Fatalf("write attribution: %v", err)
	}
}

// syncConfig points the LOD2, LOD3 and TABULA data paths at fresh folders
// holding one dataset each, and gives the test its own dataset_attribution
// table in a dedicated schema.
func syncConfig(t *testing.T, ctx context.Context) (*config.Config, string) {
	t.Helper()
	t.Chdir(projectRoot())
	cfg := testConfig()
	cfg.DB.Schemas.City2Tabula = "sync_attribution_c2t"
	schema := cfg.DB.Schemas.City2Tabula
	t.Cleanup(func() { _, _ = testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	ddl, err := os.ReadFile("sql/schema/supplementary/02_create_attribution_table.sql")
	if err != nil {
		t.Fatalf("read attribution DDL: %v", err)
	}
	if _, err := testPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+schema+`;`+
		strings.ReplaceAll(string(ddl), "{city2tabula_schema}", schema)); err != nil {
		t.Fatalf("create dataset_attribution: %v", err)
	}

	lod2, lod3, tabula := t.TempDir(), t.TempDir(), t.TempDir()
	writeAttributionFolder(t, filepath.Join(lod2, "region-a"), "xx-region-a-lod2", "Credit A")
	writeAttributionFolder(t, filepath.Join(lod3, "region-b"), "xx-region-b-lod3", "Credit B")
	tabulaFile, err := os.ReadFile("data/tabula/attribution.json")
	if err != nil {
		t.Fatalf("read TABULA attribution: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tabula, attribution.FileName), tabulaFile, 0o644); err != nil {
		t.Fatalf("write TABULA attribution: %v", err)
	}
	cfg.Data = &config.DataPaths{Lod2: lod2, Lod3: lod3, Tabula: tabula + string(filepath.Separator)}
	return cfg, lod2
}

type attributionRow struct {
	credit                string
	creditURL             *string
	insertedAt, updatedAt time.Time
}

func readAttributionRows(t *testing.T, ctx context.Context, schema string) map[string]attributionRow {
	t.Helper()
	rows, err := testPool.Query(ctx, `SELECT dataset_id, credit, credit_url, inserted_at, updated_at FROM `+schema+`.dataset_attribution`)
	if err != nil {
		t.Fatalf("query dataset_attribution: %v", err)
	}
	defer rows.Close()
	got := map[string]attributionRow{}
	for rows.Next() {
		var id string
		var r attributionRow
		if err := rows.Scan(&id, &r.credit, &r.creditURL, &r.insertedAt, &r.updatedAt); err != nil {
			t.Fatalf("scan dataset_attribution: %v", err)
		}
		got[id] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate dataset_attribution: %v", err)
	}
	return got
}

func TestSyncAttribution_InsertsThenUpdatesOnlyChangedRows(t *testing.T) {
	ctx := context.Background()
	cfg, lod2 := syncConfig(t, ctx)
	schema := cfg.DB.Schemas.City2Tabula

	n, err := importer.SyncAttribution(ctx, testPool, cfg)
	if err != nil {
		t.Fatalf("first SyncAttribution: %v", err)
	}
	first := readAttributionRows(t, ctx, schema)
	if n != 3 || len(first) != 3 {
		t.Fatalf("first sync: %d rows reported, %d stored, want 3 and 3: %v", n, len(first), first)
	}
	for _, id := range []string{"xx-region-a-lod2", "xx-region-b-lod3", "tabula-episcope"} {
		if _, ok := first[id]; !ok {
			t.Errorf("no dataset_attribution row for %s", id)
		}
	}
	if first["xx-region-a-lod2"].creditURL != nil {
		t.Errorf("absent credit_url stored as %q, want NULL", *first["xx-region-a-lod2"].creditURL)
	}

	if n, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil || n != 0 {
		t.Fatalf("unchanged files: SyncAttribution = %d, %v; want 0, nil", n, err)
	}

	writeAttributionFolder(t, filepath.Join(lod2, "region-a"), "xx-region-a-lod2", "Corrected credit A")
	if n, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil || n != 1 {
		t.Fatalf("one corrected file: SyncAttribution = %d, %v; want 1, nil", n, err)
	}
	second := readAttributionRows(t, ctx, schema)
	a, b := second["xx-region-a-lod2"], second["xx-region-b-lod3"]
	if a.credit != "Corrected credit A" {
		t.Errorf("credit after correction = %q, want %q", a.credit, "Corrected credit A")
	}
	if !a.updatedAt.After(first["xx-region-a-lod2"].updatedAt) || !a.insertedAt.Equal(first["xx-region-a-lod2"].insertedAt) {
		t.Errorf("corrected row: inserted_at %v -> %v, updated_at %v -> %v; want inserted_at kept and updated_at later",
			first["xx-region-a-lod2"].insertedAt, a.insertedAt, first["xx-region-a-lod2"].updatedAt, a.updatedAt)
	}
	if !b.updatedAt.Equal(first["xx-region-b-lod3"].updatedAt) {
		t.Errorf("unchanged row's updated_at moved from %v to %v", first["xx-region-b-lod3"].updatedAt, b.updatedAt)
	}
}

// An invalid file anywhere must leave every row as it was, so a half-edited
// set of files never produces a half-updated table.
func TestSyncAttribution_InvalidFileWritesNothing(t *testing.T) {
	ctx := context.Background()
	cfg, lod2 := syncConfig(t, ctx)
	schema := cfg.DB.Schemas.City2Tabula
	if _, err := importer.SyncAttribution(ctx, testPool, cfg); err != nil {
		t.Fatalf("first SyncAttribution: %v", err)
	}

	writeAttributionFolder(t, filepath.Join(lod2, "region-a"), "xx-region-a-lod2", "Corrected credit A")
	broken := filepath.Join(lod2, "region-c", attribution.FileName)
	writeAttributionFolder(t, filepath.Dir(broken), "xx-region-c-lod2", "Credit C")
	if err := os.WriteFile(broken, []byte(`{"schema_version": 1, "dataset_id": "xx-region-c-lod2"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := importer.SyncAttribution(ctx, testPool, cfg)
	if err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("want an error naming %s, got %v", broken, err)
	}
	rows := readAttributionRows(t, ctx, schema)
	if rows["xx-region-a-lod2"].credit != "Credit A" || len(rows) != 3 {
		t.Errorf("rows changed despite the invalid file: %v", rows)
	}
}
