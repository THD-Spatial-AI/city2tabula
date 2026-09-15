//go:build integration

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/thd-spatial-ai/city2tabula/internal/api/handler"
	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/testutil"
)

// baseConfig mirrors config.LoadBaseConfig for a test container: country-specific
// fields stay unset, since RegionConfig fills them in per request.
func baseConfig(host, port string) config.Config {
	return config.Config{
		DB: &config.DBConfig{
			Host:     host,
			Port:     port,
			Name:     "coverage_ddl_test",
			User:     testutil.TestUser,
			Password: testutil.TestPassword,
			SSLMode:  "disable",
			Tables:   &config.Tables{},
			Schemas:  &config.Schemas{City2Tabula: config.City2TabulaSchema},
		},
		Data:        &config.DataPaths{},
		CityDB:      &config.CityDB{},
		City2Tabula: &config.City2TabulaConfig{},
		Batch:       &config.BatchConfig{Threads: 2},
	}
}

func databaseExists(t *testing.T, host, port, name string) bool {
	t.Helper()
	ctx := context.Background()
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		host, port, testutil.TestUser, testutil.TestPassword)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to bootstrap DB: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("check database %s: %v", name, err)
	}
	return exists
}

// TestCoverage_UnconfiguredCountryCreatesNoDatabase covers both halves of the
// defect: a read-only GET must not provision a database, and "this country has
// no dataset" is an answer the endpoint exists to give rather than a fault.
// Returning 500 here made it indistinguishable from the service being down, and
// 500 is retryable, so callers burned backoff on a settled answer.
func TestCoverage_UnconfiguredCountryCreatesNoDatabase(t *testing.T) {
	host, port := testutil.StartPostGISAddr(t)
	h := handler.New(server.New(baseConfig(host, port)))

	const germanDB = "coverage_ddl_test_de"
	if databaseExists(t, host, port, germanDB) {
		t.Fatalf("%s exists before the request; the fixture is not clean", germanDB)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/coverage?country=germany&xmin=8.78&ymin=53.08&xmax=8.83&ymax=53.11", nil)
	rec := httptest.NewRecorder()
	h.Coverage(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (an absent dataset is an answer, not a fault); body: %s",
			rec.Code, rec.Body.String())
	}

	var got struct {
		Count      int  `json:"count"`
		Configured bool `json:"configured"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if got.Count != 0 {
		t.Errorf("count = %d, want 0", got.Count)
	}
	if got.Configured {
		t.Error("configured = true, want false so a caller can tell an absent dataset from an empty bbox")
	}

	if databaseExists(t, host, port, germanDB) {
		t.Errorf("GET /coverage created database %s; a read-only endpoint must not run DDL", germanDB)
	}
}
