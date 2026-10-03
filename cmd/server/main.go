// Command server runs City2TABULA's on-request HTTP wrapper: it triggers
// bbox-scoped pipeline runs per country instead of the CLI's single
// COUNTRY-per-invocation model. See internal/api and internal/onrequest.
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/api/handler"
	"github.com/thd-spatial-ai/city2tabula/internal/api/router"
	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
	"github.com/thd-spatial-ai/city2tabula/internal/version"
)

func main() {
	utils.InitLogger()
	// Before config validation, not after: a container that fails to start still
	// has to say which build produced the failure. Values come from -ldflags at
	// release time and are otherwise "dev".
	utils.Info.Printf("City2TABULA %s (commit %s, built %s)",
		version.Version, version.Commit, version.Date)

	base := config.LoadBaseConfig()
	if err := validateBaseConfig(base); err != nil {
		utils.Error.Fatalf("Invalid configuration: %v", err)
	}

	interval, err := attributionCheckInterval()
	if err != nil {
		utils.Error.Fatalf("Invalid configuration: %v", err)
	}

	srv := server.New(base)
	srv.StartAttributionChecks(context.Background(), interval)
	h := handler.New(srv)

	addr := ":" + config.GetEnv("SERVER_PORT", "5000")
	utils.Info.Printf("City2TABULA on-request server listening on %s", addr)
	utils.Error.Fatal(http.ListenAndServe(addr, router.New(h)))
}

// attributionCheckInterval reads ATTRIBUTION_CHECK_INTERVAL, a Go duration such
// as 168h, defaulting to weekly. 0 disables the scheduled check.
func attributionCheckInterval() (time.Duration, error) {
	raw := config.GetEnv("ATTRIBUTION_CHECK_INTERVAL", "168h")
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("ATTRIBUTION_CHECK_INTERVAL %q is not a duration of 0 or more, such as 168h", raw)
	}
	return d, nil
}

// validateBaseConfig checks process-wide settings only — country-specific
// ones are resolved per request, so config.Config.Validate() doesn't apply.
func validateBaseConfig(cfg config.Config) error {
	missing := []string{}
	if strings.TrimSpace(cfg.DB.Host) == "" {
		missing = append(missing, "DB_HOST")
	}
	if strings.TrimSpace(cfg.DB.User) == "" {
		missing = append(missing, "DB_USER")
	}
	if strings.TrimSpace(cfg.DB.Password) == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if strings.TrimSpace(cfg.CityDB.ToolPath) == "" {
		missing = append(missing, "CITYDB_TOOL_PATH")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
}
