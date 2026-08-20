// Package server holds the HTTP wrapper's shared state: cached per-country DB
// pools and in-memory run tracking. See internal/onrequest for the pipeline
// logic this drives, and internal/api/handler for the HTTP layer on top.
package server

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// Run statuses.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusNoData    = "no_data" // pipeline ran fine but found no source data for the bbox
	StatusFailed    = "failed"
)

// Run tracks one triggered pipeline run. Held in memory only — acceptable for
// the MVP since the ground truth after a successful run is building_link itself;
// a lost run record just means the caller has to re-check coverage and retrigger.
type Run struct {
	ID        string
	Country   string
	Status    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Server holds state shared across requests: one DB pool per country (opened
// lazily, kept for reuse by read queries) and in-memory run tracking.
type Server struct {
	base config.Config

	// runMu serializes pipeline runs across all countries.
	// ponytail: single global run-queue, not per-region — upgrade to per-region
	// locking only if concurrent on-request runs actually create a backlog.
	runMu sync.Mutex

	poolsMu sync.Mutex
	pools   map[string]*pgxpool.Pool

	runsMu sync.RWMutex
	runs   map[string]*Run
}

// New builds a Server from the process-wide base config (see config.LoadBaseConfig).
func New(base config.Config) *Server {
	return &Server{
		base:  base,
		pools: make(map[string]*pgxpool.Pool),
		runs:  make(map[string]*Run),
	}
}

// PoolFor returns the region config and a cached, lazily-opened DB pool for
// country. Used by read-only endpoints (coverage, buildings) so repeat requests
// for the same country reuse one connection pool instead of reconnecting.
func (s *Server) PoolFor(country string) (*config.Config, *pgxpool.Pool, error) {
	cfg, err := config.RegionConfig(s.base, country)
	if err != nil {
		return nil, nil, err
	}

	s.poolsMu.Lock()
	defer s.poolsMu.Unlock()

	if pool, ok := s.pools[cfg.Country]; ok {
		return &cfg, pool, nil
	}

	pool, err := db.ConnectPool(&cfg)
	if err != nil {
		return nil, nil, err
	}
	s.pools[cfg.Country] = pool
	return &cfg, pool, nil
}

// StartRun validates country/bbox, registers a pending Run, and kicks off the
// pipeline in a background goroutine. Returns the run's id immediately — callers
// poll GetRun for progress.
func (s *Server) StartRun(country string, bbox onrequest.Bbox, bboxMode string) (*Run, error) {
	cfg, err := config.RegionConfig(s.base, country)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	run := &Run{ID: uuid.NewString(), Country: cfg.Country, Status: StatusPending, CreatedAt: now, UpdatedAt: now}

	s.runsMu.Lock()
	s.runs[run.ID] = run
	s.runsMu.Unlock()

	go s.executeRun(run.ID, cfg, bbox, bboxMode)

	return run, nil
}

// GetRun returns the current state of a previously started run.
func (s *Server) GetRun(id string) (*Run, bool) {
	s.runsMu.RLock()
	defer s.runsMu.RUnlock()
	run, ok := s.runs[id]
	return run, ok
}

func (s *Server) executeRun(id string, cfg config.Config, bbox onrequest.Bbox, bboxMode string) {
	s.setRunStatus(id, StatusRunning, "")

	s.runMu.Lock()
	defer s.runMu.Unlock()

	if err := onrequest.RunForRegion(&cfg, bbox, bboxMode); err != nil {
		utils.Error.Printf("on-request run %s (%s) failed: %v", id, cfg.Country, err)
		s.setRunStatus(id, StatusFailed, err.Error())
		return
	}

	_, pool, err := s.PoolFor(cfg.Country)
	if err != nil {
		s.setRunStatus(id, StatusFailed, err.Error())
		return
	}

	count, err := onrequest.CountBuildingLink(context.Background(), pool, &cfg, bbox)
	if err != nil {
		s.setRunStatus(id, StatusFailed, err.Error())
		return
	}
	if count == 0 {
		utils.Warn.Printf("on-request run %s (%s) found no source data for bbox %s", id, cfg.Country, bbox)
		s.setRunStatus(id, StatusNoData, "")
		return
	}

	s.setRunStatus(id, StatusCompleted, "")
}

func (s *Server) setRunStatus(id, status, errMsg string) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return
	}
	run.Status = status
	run.Error = errMsg
	run.UpdatedAt = time.Now().UTC()
}
