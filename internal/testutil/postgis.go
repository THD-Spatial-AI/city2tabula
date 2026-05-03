//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgisImage = "postgis/postgis:17-3.4"
	testDBName   = "city2tabula_test"
	testUser     = "test"
	testPassword = "test"
)

// StartPostGIS starts a PostGIS container and returns a connected pool.
// The container and pool are stopped automatically when the test ends via t.Cleanup.
func StartPostGIS(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: postgisImage,
		Env: map[string]string{
			"POSTGRES_DB":       testDBName,
			"POSTGRES_USER":     testUser,
			"POSTGRES_PASSWORD": testPassword,
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").AsRegexp(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start PostGIS container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate PostGIS container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		testUser, testPassword, host, port.Port(), testDBName,
	)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create connection pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// ConnectionString returns the DSN for the running container.
// Useful for TestMain setups where t is not available.
func ContainerConnString(host, port string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", testUser, testPassword, host, port, testDBName)
}
