//go:build integration

package onrequest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// TestMissingRunSchemas_FixtureShapedDatabase covers a database restored from
// the published fixture: it holds the City2TABULA schema and nothing else, so an
// incremental import would fail on tabula.tabula rather than on anything that
// names the real cause.
func TestMissingRunSchemas_FixtureShapedDatabase(t *testing.T) {
	pool := testutil.StartPostGIS(t)
	if _, err := pool.Exec(context.Background(),
		`CREATE SCHEMA IF NOT EXISTS `+config.City2TabulaSchema); err != nil {
		t.Fatalf("create city2tabula schema: %v", err)
	}

	cfg := &config.Config{
		Country: "netherlands",
		DB: &config.DBConfig{
			Name: "fixture_shaped",
			Schemas: &config.Schemas{
				City2Tabula: config.City2TabulaSchema,
				Tabula:      config.TabulaSchema,
				CityDB:      config.CityDBSchema,
				Lod2:        config.Lod2Schema,
			},
		},
	}

	missing, err := db.MissingRunSchemas(pool, cfg)
	if err != nil {
		t.Fatalf("MissingRunSchemas: %v", err)
	}

	got := strings.Join(missing, ",")
	for _, want := range []string{config.TabulaSchema, config.CityDBSchema, config.Lod2Schema} {
		if !strings.Contains(got, want) {
			t.Errorf("missing schemas %q does not include %q", got, want)
		}
	}
	if strings.Contains(got, config.City2TabulaSchema) {
		t.Errorf("missing schemas %q lists %q, which exists", got, config.City2TabulaSchema)
	}
}

// TestMissingRunSchemas_FullyProvisionedDatabase is the other side: a database
// with every schema a run writes into reports nothing missing, so the check
// cannot block a real incremental import.
func TestMissingRunSchemas_FullyProvisionedDatabase(t *testing.T) {
	pool := testutil.StartPostGIS(t)
	for _, schema := range []string{
		config.City2TabulaSchema, config.TabulaSchema, config.CityDBSchema, config.Lod2Schema,
	} {
		if _, err := pool.Exec(context.Background(), `CREATE SCHEMA IF NOT EXISTS `+schema); err != nil {
			t.Fatalf("create schema %s: %v", schema, err)
		}
	}

	cfg := &config.Config{
		Country: "netherlands",
		DB: &config.DBConfig{
			Name: "fully_provisioned",
			Schemas: &config.Schemas{
				City2Tabula: config.City2TabulaSchema,
				Tabula:      config.TabulaSchema,
				CityDB:      config.CityDBSchema,
				Lod2:        config.Lod2Schema,
			},
		},
	}

	missing, err := db.MissingRunSchemas(pool, cfg)
	if err != nil {
		t.Fatalf("MissingRunSchemas: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("expected nothing missing, got %v", missing)
	}
}
