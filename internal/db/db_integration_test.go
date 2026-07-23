//go:build integration

package db_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testHost/testPort point at the bootstrap "postgres" database in the container;
// each test builds its own *config.Config from these so EnsureDatabase/ConnectPool
// (the functions under test) are the only thing creating/opening the target DB.
var (
	testHost string
	testPort string
)

// testPool is a shared connection, opened once via db.ConnectPool against a
// dedicated test database, for tests that only need a working pool (schema
// create/drop, ListCityDBSchemas) rather than testing connection setup itself.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "postgis/postgis:17-3.4",
		Env: map[string]string{
			"POSTGRES_DB":       "postgres",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").AsRegexp(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start PostGIS container: %v", err)
	}
	defer container.Terminate(ctx)

	testHost, err = container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("failed to get container port: %v", err)
	}
	testPort = port.Port()

	// PostGIS images do extra initialization after the "ready" log line. Ping until
	// PostgreSQL is truly accepting connections (up to 15 seconds) before the tests'
	// own db.ConnectPool call, which would otherwise race that restart.
	if err := waitForBootstrapDB(ctx); err != nil {
		log.Fatalf("PostgreSQL not ready: %v", err)
	}

	pool, err := db.ConnectPool(testConfig("citytabula_dbtest"))
	if err != nil {
		log.Fatalf("failed to connect shared test pool: %v", err)
	}
	testPool = pool
	defer db.ClosePool(testPool)

	os.Exit(m.Run())
}

// waitForBootstrapDB pings the bootstrap "postgres" database until it accepts
// connections or 15 seconds pass, mirroring internal/process/integration_test.go's
// TestMain (same PostGIS image, same post-"ready"-log restart to wait out).
func waitForBootstrapDB(ctx context.Context) error {
	bootstrapDSN := fmt.Sprintf("host=%s port=%s user=test password=test dbname=postgres sslmode=disable", testHost, testPort)
	pool, err := pgxpool.New(ctx, bootstrapDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	var lastErr error
	for i := 0; i < 30; i++ {
		if lastErr = pool.Ping(ctx); lastErr == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

// testConfig builds the minimal *config.Config the functions under test need:
// connection details plus the two schema names ListCityDBSchemas filters on.
func testConfig(dbName string) *config.Config {
	return &config.Config{
		DB: &config.DBConfig{
			Host:     testHost,
			Port:     testPort,
			Name:     dbName,
			User:     "test",
			Password: "test",
			SSLMode:  "disable",
			Schemas: &config.Schemas{
				Lod2: "lod2",
				Lod3: "lod3",
			},
		},
		Batch: &config.BatchConfig{Threads: 2},
	}
}

func TestEnsureDatabase_CreatesDatabaseIfMissing(t *testing.T) {
	cfg := testConfig("ensure_db_test")

	if err := db.EnsureDatabase(cfg); err != nil {
		t.Fatalf("EnsureDatabase (create): %v", err)
	}

	exists, err := databaseExists(t, cfg.DB.Name)
	if err != nil {
		t.Fatalf("failed to check database existence: %v", err)
	}
	if !exists {
		t.Fatalf("expected database %q to exist after EnsureDatabase, it does not", cfg.DB.Name)
	}

	// Idempotent: calling it again against an already-existing database must not error.
	if err := db.EnsureDatabase(cfg); err != nil {
		t.Fatalf("EnsureDatabase (already exists): %v", err)
	}
}

func databaseExists(t *testing.T, name string) (bool, error) {
	t.Helper()
	bootstrapDSN := fmt.Sprintf("host=%s port=%s user=test password=test dbname=postgres sslmode=disable", testHost, testPort)
	conn, err := pgxpool.New(context.Background(), bootstrapDSN)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	var exists bool
	err = conn.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name,
	).Scan(&exists)
	return exists, err
}

func TestConnectPool_ConnectsAndEnablesPostGIS(t *testing.T) {
	cfg := testConfig("connect_pool_test")

	pool, err := db.ConnectPool(cfg)
	if err != nil {
		t.Fatalf("ConnectPool: %v", err)
	}
	defer db.ClosePool(pool)

	var version string
	if err := pool.QueryRow(context.Background(), `SELECT PostGIS_Version()`).Scan(&version); err != nil {
		t.Errorf("expected PostGIS to be enabled on the connected database, PostGIS_Version() failed: %v", err)
	}
}

func TestCreateAndDropSchemaIfNotExists_AreIdempotent(t *testing.T) {
	ctx := context.Background()
	const schema = "create_drop_idempotent_test"
	defer testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)

	if err := db.CreateSchemaIfNotExists(testPool, schema); err != nil {
		t.Fatalf("CreateSchemaIfNotExists (1st): %v", err)
	}
	if err := db.CreateSchemaIfNotExists(testPool, schema); err != nil {
		t.Fatalf("CreateSchemaIfNotExists (2nd, already exists): %v", err)
	}
	if !schemaExists(t, ctx, schema) {
		t.Fatalf("expected schema %q to exist after CreateSchemaIfNotExists", schema)
	}

	if err := db.DropSchemaIfExists(testPool, schema); err != nil {
		t.Fatalf("DropSchemaIfExists (1st): %v", err)
	}
	if err := db.DropSchemaIfExists(testPool, schema); err != nil {
		t.Fatalf("DropSchemaIfExists (2nd, already gone): %v", err)
	}
	if schemaExists(t, ctx, schema) {
		t.Fatalf("expected schema %q to be gone after DropSchemaIfExists", schema)
	}
}

func schemaExists(t *testing.T, ctx context.Context, name string) bool {
	t.Helper()
	var exists bool
	if err := testPool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("failed to check schema existence for %q: %v", name, err)
	}
	return exists
}

func TestCreateSchemas_CreatesEveryListedSchema(t *testing.T) {
	ctx := context.Background()
	schemas := []string{"batch_create_a_test", "batch_create_b_test"}
	defer func() {
		for _, s := range schemas {
			testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+s+` CASCADE`)
		}
	}()

	if err := db.CreateSchemas(testPool, schemas); err != nil {
		t.Fatalf("CreateSchemas: %v", err)
	}
	for _, s := range schemas {
		if !schemaExists(t, ctx, s) {
			t.Errorf("expected schema %q to exist after CreateSchemas", s)
		}
	}
}

func TestDropCity2TabulaSchemas_DropsEveryListedSchema(t *testing.T) {
	ctx := context.Background()
	schemas := []string{"batch_drop_a_test", "batch_drop_b_test"}
	if err := db.CreateSchemas(testPool, schemas); err != nil {
		t.Fatalf("failed to seed schemas to drop: %v", err)
	}

	if err := db.DropCity2TabulaSchemas(testPool, schemas); err != nil {
		t.Fatalf("DropCity2TabulaSchemas: %v", err)
	}
	for _, s := range schemas {
		if schemaExists(t, ctx, s) {
			t.Errorf("expected schema %q to be gone after DropCity2TabulaSchemas", s)
		}
	}
}

// TestListCityDBSchemas_FindsConfiguredAndCitydbLikeSchemas guards against the bug
// where the query was built via fmt.Sprintf (schema names already inlined, no
// placeholders) while conn.Query was still called with those same two values as
// extra positional args - pgx rejects a query with unused/mismatched arguments, so
// this call failed outright before the fix. Now fixed to parameterize once.
func TestListCityDBSchemas_FindsConfiguredAndCitydbLikeSchemas(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig("connect_pool_test") // Schemas.Lod2="lod2", Schemas.Lod3="lod3"

	matching := []string{"lod2", "lod3", "citydb_pkg_list_test"}
	unrelated := "unrelated_schema_list_test"
	defer func() {
		for _, s := range append(matching, unrelated) {
			testPool.Exec(ctx, `DROP SCHEMA IF EXISTS `+s+` CASCADE`)
		}
	}()
	if err := db.CreateSchemas(testPool, append(matching, unrelated)); err != nil {
		t.Fatalf("failed to seed schemas: %v", err)
	}

	got, err := db.ListCityDBSchemas(testPool, cfg)
	if err != nil {
		t.Fatalf("ListCityDBSchemas: %v", err)
	}

	gotSet := make(map[string]bool, len(got))
	for _, s := range got {
		gotSet[s] = true
	}
	for _, want := range matching {
		if !gotSet[want] {
			t.Errorf("expected ListCityDBSchemas to include %q, got %v", want, got)
		}
	}
	if gotSet[unrelated] {
		t.Errorf("expected ListCityDBSchemas to exclude %q, got %v", unrelated, got)
	}
}
