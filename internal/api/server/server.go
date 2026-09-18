// Package server holds the HTTP wrapper's shared state: cached per-country DB
// pools and in-memory run tracking. See internal/onrequest for the pipeline
// logic this drives, and internal/api/handler for the HTTP layer on top.
package server

import (
	"context"
	"fmt"
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

// activeRun is one pipeline run currently holding the region it processes.
type activeRun struct {
	dbName string
	bbox   onrequest.Bbox
}

// runKey identifies a request precisely enough that two requests sharing it
// would do identical work. Bbox.String is used rather than the float struct
// because it is already the canonical form sent to citydb-tool.
func runKey(dbName string, bbox onrequest.Bbox, bboxMode string) string {
	return dbName + "|" + bbox.String() + "|" + bboxMode
}

// Server holds state shared across requests: one DB pool per country (opened
// lazily, kept for reuse by read queries) and in-memory run tracking.
type Server struct {
	base config.Config

	// Pipeline runs are serialized only where they can collide: same database
	// and overlapping bbox. Two runs over disjoint regions, or over different
	// countries, touch different rows and proceed in parallel.
	//
	// The skip logic in the SQL (scripts 01 and 08) stops a building being
	// stored twice, but not two workers processing it at once, which is what
	// this guards.
	activeMu   sync.Mutex
	activeCond *sync.Cond
	active     []activeRun
	// inflight maps a run key to the run already serving it, so identical
	// requests join rather than duplicate the work. Entries live until the run
	// reaches a terminal status.
	inflight map[string]string

	poolsMu sync.Mutex
	pools   map[string]*pgxpool.Pool

	runsMu sync.RWMutex
	runs   map[string]*Run
}

// New builds a Server from the process-wide base config (see config.LoadBaseConfig).
func New(base config.Config) *Server {
	srv := &Server{
		base:     base,
		pools:    make(map[string]*pgxpool.Pool),
		runs:     make(map[string]*Run),
		inflight: make(map[string]string),
	}
	srv.activeCond = sync.NewCond(&srv.activeMu)
	return srv
}

// PoolFor returns the region config and a cached, lazily-opened DB pool for
// country. Used by read-only endpoints (coverage, buildings) so repeat requests
// for the same country reuse one connection pool instead of reconnecting.
//
// Returns db.ErrCountryNotConfigured for a country with no usable dataset rather
// than creating one: ConnectPool creates the database as a side effect, so
// without this check a GET would provision an empty database for any country
// name it is handed. A database that exists but has no City2TABULA tables counts
// as unconfigured too, since earlier builds left such databases behind.
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

	exists, err := db.DatabaseExists(&cfg)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, fmt.Errorf("%s: %w", cfg.Country, db.ErrCountryNotConfigured)
	}

	pool, err := db.ConnectPool(&cfg)
	if err != nil {
		return nil, nil, err
	}

	provisioned, err := db.CountryProvisioned(pool, &cfg)
	if err != nil {
		db.ClosePool(pool)
		return nil, nil, err
	}
	if !provisioned {
		// Not cached: the country becomes usable once a run provisions it, and a
		// cached pool would keep answering "not configured" until a restart.
		db.ClosePool(pool)
		return nil, nil, fmt.Errorf("%s: %w", cfg.Country, db.ErrCountryNotConfigured)
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

	// Checked before the run is registered: accepting with 202 and failing in
	// the goroutine leaves the caller polling a run that was never going to
	// succeed.
	if err := onrequest.CheckRunnable(&cfg); err != nil {
		return nil, err
	}

	key := runKey(cfg.DB.Name, bbox, bboxMode)

	s.activeMu.Lock()
	if id, ok := s.inflight[key]; ok {
		s.activeMu.Unlock()
		// An identical request is already being served. Returning that run
		// rather than starting a second one means both callers poll the same
		// id and the region is processed once.
		if existing, found := s.GetRun(id); found {
			utils.Info.Printf("on-request run %s (%s) joined by an identical request for %s", id, cfg.Country, bbox)
			return existing, nil
		}
		// The run record vanished without clearing its key; fall through and
		// start a fresh one rather than returning nothing.
		s.activeMu.Lock()
		delete(s.inflight, key)
	}

	now := time.Now().UTC()
	run := &Run{ID: uuid.NewString(), Country: cfg.Country, Status: StatusPending, CreatedAt: now, UpdatedAt: now}
	s.inflight[key] = run.ID
	s.activeMu.Unlock()

	s.runsMu.Lock()
	s.runs[run.ID] = run
	s.runsMu.Unlock()

	go s.executeRun(run.ID, cfg, bbox, bboxMode, key)

	return run, nil
}

// GetRun returns the current state of a previously started run.
func (s *Server) GetRun(id string) (*Run, bool) {
	s.runsMu.RLock()
	defer s.runsMu.RUnlock()
	run, ok := s.runs[id]
	return run, ok
}

func (s *Server) executeRun(id string, cfg config.Config, bbox onrequest.Bbox, bboxMode string, key string) {
	// The run stays pending while it waits for a conflicting region, so a
	// caller polling can tell waiting from working.
	s.acquireRegion(cfg.DB.Name, bbox)
	defer s.releaseRegion(cfg.DB.Name, bbox, key)

	s.setRunStatus(id, StatusRunning, "")

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

// acquireRegion blocks until no active run covers ground that bbox also
// covers in the same database, then registers this run as holding it.
func (s *Server) acquireRegion(dbName string, bbox onrequest.Bbox) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	for s.conflictsLocked(dbName, bbox) {
		s.activeCond.Wait()
	}
	s.active = append(s.active, activeRun{dbName: dbName, bbox: bbox})
}

// releaseRegion drops this run's hold and its dedup key, and wakes anything
// waiting on an overlapping region.
func (s *Server) releaseRegion(dbName string, bbox onrequest.Bbox, key string) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	for i, a := range s.active {
		if a.dbName == dbName && a.bbox == bbox {
			s.active = append(s.active[:i], s.active[i+1:]...)
			break
		}
	}
	delete(s.inflight, key)
	s.activeCond.Broadcast()
}

// conflictsLocked reports whether an active run would touch the same buildings.
// Callers must hold activeMu.
func (s *Server) conflictsLocked(dbName string, bbox onrequest.Bbox) bool {
	for _, a := range s.active {
		if a.dbName == dbName && a.bbox.Overlaps(bbox) {
			return true
		}
	}
	return false
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
