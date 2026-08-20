//go:build integration

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgisImage = "postgis/postgis:17-3.4"
	testDBName   = "city2tabula_test"

	// TestUser/TestPassword are exported so callers that need to build their own
	// config.Config (e.g. to point at a different database on the same server,
	// via StartPostGISAddr) can fill in DB.User/DB.Password without duplicating
	// these values.
	TestUser     = "test"
	TestPassword = "test"
)

// StartPostGIS starts a PostGIS container and returns a connected pool to its
// default database. The container and pool are stopped automatically when the
// test ends via t.Cleanup.
func StartPostGIS(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host, port := StartPostGISAddr(t)

	pool, err := pgxpool.New(context.Background(), ContainerConnString(host, port))
	if err != nil {
		t.Fatalf("failed to create connection pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// StartPostGISAddr starts a PostGIS container and returns its host/port, without
// opening a pool — for tests that need to connect to more than one database on
// the same server (e.g. building their own config.Config per database, the way
// the on-request server opens one database per country).
func StartPostGISAddr(t *testing.T) (host, port string) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: postgisImage,
		Env: map[string]string{
			"POSTGRES_DB":       testDBName,
			"POSTGRES_USER":     TestUser,
			"POSTGRES_PASSWORD": TestPassword,
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

	h, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	p, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}
	port = p.Port()

	waitUntilReady(t, ctx, h, port)

	return h, port
}

// waitUntilReady pings the container's default database until it accepts
// connections. PostGIS images do extra initialization (running init scripts,
// then restarting the server) after the "ready" log line StartPostGISAddr
// already waits for, so a connection attempt right after that log line can
// still be refused or reset.
func waitUntilReady(t *testing.T, ctx context.Context, host, port string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, ContainerConnString(host, port))
	if err != nil {
		t.Fatalf("failed to create readiness-check pool: %v", err)
	}
	defer pool.Close()

	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := pool.Ping(ctx); err == nil {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("PostGIS container not ready after 15s: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ContainerConnString returns the DSN for the running container's default database.
// Useful for TestMain setups where t is not available.
func ContainerConnString(host, port string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", TestUser, TestPassword, host, port, testDBName)
}
