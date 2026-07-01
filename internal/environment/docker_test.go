//go:build integration

// Package environment_test builds and runs the actual environment/Dockerfile
// image used by `make build` / `docker compose build`, and asserts the
// container's runtime config comes from the environment docker-compose
// injects (env_file: docker.env), never from a file baked into the image.
//
// This guards against a regression that broke the setup for every Docker
// Desktop user (Windows and macOS): a developer's own host-side .env
// (DB_HOST=localhost, meant for running the binary directly on the host)
// got copied into the image by `COPY . .` because no .dockerignore excluded
// it. internal/config.LoadEnv() calls godotenv.Overload(), which makes an
// in-image .env win over already-set container env vars — so docker.env's
// DB_HOST=host.docker.internal was silently discarded in favour of the
// baked-in localhost value. On Linux that just means "connects to the wrong
// place"; on Windows/macOS Docker Desktop, host.docker.internal is the only
// way to reach a host-side Postgres at all (there is no host network
// passthrough), so the container fails outright with a connection-refused
// error that looks like a Postgres problem, not a config problem.
package environment_test

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// environmentTestImage is a fixed, known image reference, set once by
// TestMain and reused read-only by every test below.
const environmentTestImage = "city2tabula-environment-integration-test:latest"

// TestMain builds the real environment/Dockerfile once for the whole
// package run, exactly as `docker compose build` does. Building once (rather
// than per-test) matters here: this image installs Go, Oracle JDK, and
// citydb-tool from the network, so a second from-scratch build can blow past
// a per-test timeout even though it would hit Docker's layer cache.
func TestMain(m *testing.M) {
	flag.Parse() // must run before testing.Verbose() is usable

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../..", // project root, matches `context: ./..` in environment/docker-compose.yml
			Dockerfile:    "environment/Dockerfile",
			Repo:          "city2tabula-environment-integration-test",
			Tag:           "latest",
			KeepImage:     true,
			PrintBuildLog: testing.Verbose(),
		},
		Cmd: []string{"true"}, // exits immediately; we only need the image built
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to build environment image: %v", err)
	}
	// KeepImage: true means Terminate won't delete the image, only this
	// throwaway build container.
	defer container.Terminate(context.Background())

	os.Exit(m.Run())
}

// TestDockerfile_DoesNotBakeInHostEnvFile fails if the image contains a .env
// file. A baked-in .env is what caused godotenv.Overload() to silently
// discard docker-compose's env_file values (see package doc comment).
func TestDockerfile_DoesNotBakeInHostEnvFile(t *testing.T) {
	image := environmentTestImage

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      image,
			Cmd:        []string{"bash", "-c", "test -f .env && echo FOUND || echo MISSING"},
			WaitingFor: nil,
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	state, err := container.State(ctx)
	if err == nil && state.Running {
		// Give the short-lived command a moment to exit before reading logs.
		time.Sleep(500 * time.Millisecond)
	}

	logsReader, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("failed to read container logs: %v", err)
	}
	defer logsReader.Close()

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, logsReader); err != nil {
		t.Fatalf("failed to drain container logs: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "FOUND") {
		t.Fatalf(".env file was baked into the image (should be excluded by .dockerignore); container output: %q", out)
	}
	if !strings.Contains(out, "MISSING") {
		t.Fatalf("unexpected container output, expected MISSING: %q", out)
	}
}

// TestDockerfile_UsesInjectedEnvNotBakedConfig proves that runtime config
// (DB_HOST in particular) comes from whatever the caller injects via
// container env vars — the same mechanism docker-compose's `env_file:
// docker.env` uses — and not from any file baked into the image at build
// time. It sets DB_HOST to a canary value that cannot resolve, runs
// `-create-db`, and asserts the connection failure names the canary host.
// If a baked-in .env were silently overriding it again, the error would
// instead name whatever host that stale file specifies (e.g. "localhost").
func TestDockerfile_UsesInjectedEnvNotBakedConfig(t *testing.T) {
	image := environmentTestImage

	const canaryHost = "ci-canary-host-should-not-be-overridden"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: image,
			Env: map[string]string{
				"COUNTRY":         "netherlands",
				"DB_HOST":         canaryHost,
				"DB_PORT":         "5432",
				"DB_USER":         "postgres",
				"DB_PASSWORD":     "postgres",
				"DB_NAME":         "irrelevant_for_this_test",
				"DB_SSL_MODE":     "disable",
				"CITYDB_SRID":     "28992",
				"CITYDB_SRS_NAME": "Amersfoort / RD New",
			},
			Cmd: []string{"./city2tabula", "-create-db"},
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	// The canary host doesn't resolve, so city2tabula fails fast trying to
	// connect. Poll logs until the process has produced output or timed out.
	var out string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		logsReader, err := container.Logs(ctx)
		if err != nil {
			t.Fatalf("failed to read container logs: %v", err)
		}
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, logsReader)
		logsReader.Close()
		out = buf.String()

		if strings.Contains(out, canaryHost) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !strings.Contains(out, canaryHost) {
		t.Fatalf("expected connection attempt to name injected DB_HOST %q; got logs: %q", canaryHost, out)
	}
}
