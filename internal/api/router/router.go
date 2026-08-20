// Package router wires the on-request HTTP wrapper's routes to their handlers.
package router

import (
	"net/http"

	"github.com/thd-spatial-ai/city2tabula/internal/api/handler"
)

// New builds the HTTP handler for the City2TABULA on-request server:
// GET /api/v1/health (liveness),
// POST /api/v1/runs (trigger a run),
// GET /api/v1/runs/{id} (run status),
// GET /api/v1/coverage (pre-trigger check),
// GET /api/v1/buildings (thematic 3D attributes, no geometry),
// GET /api/v1/geometry (footprint geometry, fetched separately).
func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", h.Health)
	mux.HandleFunc("POST /api/v1/runs", h.Runs)
	mux.HandleFunc("GET /api/v1/runs/{id}", h.RunStatus)
	mux.HandleFunc("GET /api/v1/coverage", h.Coverage)
	mux.HandleFunc("GET /api/v1/buildings", h.Buildings)
	mux.HandleFunc("GET /api/v1/geometry", h.Geometry)
	return mux
}
