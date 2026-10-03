package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/importer"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// CheckAttributionURLs runs the attribution URL check on every country database
// this server can serve and logs each failure as a warning. A country without a
// database is skipped; one whose check cannot run is logged and the rest are
// still checked. Nothing is withheld because a URL failed.
func (s *Server) CheckAttributionURLs(ctx context.Context, client *http.Client) {
	for _, country := range config.SupportedCountries() {
		cfg, pool, err := s.PoolFor(country)
		if errors.Is(err, db.ErrCountryNotConfigured) {
			continue
		}
		if err != nil {
			utils.Warn.Printf("Attribution URL check skipped for %s: %v", country, err)
			continue
		}
		failures, err := importer.CheckAttributionURLs(ctx, pool, cfg.DB.Schemas.City2Tabula, client)
		if err != nil {
			utils.Warn.Printf("Attribution URL check failed to run on %s: %v", cfg.DB.Name, err)
			continue
		}
		for _, f := range failures {
			utils.Warn.Printf("Attribution URL failed on %s: %s %s: %v", cfg.DB.Name, f.DatasetID, f.Field, f.Err)
		}
	}
}

// StartAttributionChecks runs CheckAttributionURLs at once and then every
// interval until ctx ends. An interval of 0 disables the checks.
func (s *Server) StartAttributionChecks(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		utils.Info.Println("Scheduled attribution URL checks disabled")
		return
	}
	go func() {
		client := attribution.NewURLCheckClient()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			s.CheckAttributionURLs(ctx, client)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
