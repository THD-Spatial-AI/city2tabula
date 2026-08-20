// Package handler implements the on-request HTTP wrapper's handlers: trigger a
// City2TABULA run for a country/bbox, poll its status, and query already-linked
// 3D building data. See internal/api/server for the state these operate on.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// Handler holds the dependencies shared by all HTTP handlers.
type Handler struct {
	srv *server.Server
}

// New creates a Handler bound to the given Server.
func New(srv *server.Server) *Handler {
	return &Handler{srv: srv}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Health handles GET /api/v1/health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type runRequest struct {
	Country  string  `json:"country"`
	Xmin     float64 `json:"xmin"`
	Ymin     float64 `json:"ymin"`
	Xmax     float64 `json:"xmax"`
	Ymax     float64 `json:"ymax"`
	BboxMode string  `json:"bbox_mode"`
}

type runResponse struct {
	RunID   string `json:"run_id"`
	Country string `json:"country"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func toRunResponse(run *server.Run) runResponse {
	return runResponse{RunID: run.ID, Country: run.Country, Status: run.Status, Error: run.Error}
}

// Runs handles POST /api/v1/runs: triggers a City2TABULA run for one
// country/bbox and returns immediately with a run id to poll.
func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "can't parse request body: "+err.Error())
		return
	}
	if req.Country == "" {
		writeError(w, http.StatusBadRequest, "country is required")
		return
	}
	if req.BboxMode == "" {
		req.BboxMode = "intersects"
	}

	bbox := onrequest.Bbox{Xmin: req.Xmin, Ymin: req.Ymin, Xmax: req.Xmax, Ymax: req.Ymax}
	run, err := h.srv.StartRun(req.Country, bbox, req.BboxMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, toRunResponse(run))
}

// RunStatus handles GET /api/v1/runs/{id}.
func (h *Handler) RunStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, ok := h.srv.GetRun(id)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown run id: "+id)
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// Coverage handles GET /api/v1/coverage?country=..&xmin=..&ymin=..&xmax=..&ymax=..
// — a read-only count of already-linked buildings in the bbox, so callers can
// decide whether to trigger a run before doing so.
func (h *Handler) Coverage(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	if country == "" {
		writeError(w, http.StatusBadRequest, "country query param is required")
		return
	}
	bbox, err := parseBboxParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg, pool, err := h.srv.PoolFor(country)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	count, err := onrequest.CountBuildingLink(r.Context(), pool, cfg, bbox)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

// Buildings handles GET /api/v1/buildings?country=..&osm_ids=a,b,c (3D
// attributes for already PyLovo-linked buildings, keyed by osm_id) or
// GET /api/v1/buildings?country=..&xmin=..&ymin=..&xmax=..&ymax=.. (every
// building in a bbox, independent of PyLovo linkage). osm_ids takes
// precedence if both are present.
func (h *Handler) Buildings(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	if country == "" {
		writeError(w, http.StatusBadRequest, "country query param is required")
		return
	}

	cfg, pool, err := h.srv.PoolFor(country)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var buildings []onrequest.Building
	if osmIDsParam := r.URL.Query().Get("osm_ids"); osmIDsParam != "" {
		buildings, err = onrequest.BuildingsByOSMIDs(r.Context(), pool, cfg, strings.Split(osmIDsParam, ","))
	} else if bbox, bboxErr := parseBboxParams(r); bboxErr == nil {
		buildings, err = onrequest.BuildingsByBBox(r.Context(), pool, cfg, bbox)
	} else {
		writeError(w, http.StatusBadRequest, "osm_ids or xmin/ymin/xmax/ymax query params are required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, buildings)
}

// Geometry handles GET /api/v1/geometry?country=..&object_ids=a,b,c — footprint
// geometry for the given buildings. Separate from Buildings since nothing in
// the calculation path needs geometry; fetched only when something (e.g. a
// frontend) actually wants to render it.
func (h *Handler) Geometry(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	if country == "" {
		writeError(w, http.StatusBadRequest, "country query param is required")
		return
	}
	objectIDsParam := r.URL.Query().Get("object_ids")
	if objectIDsParam == "" {
		writeError(w, http.StatusBadRequest, "object_ids query param is required")
		return
	}

	cfg, pool, err := h.srv.PoolFor(country)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	geometry, err := onrequest.BuildingGeometryByObjectIDs(r.Context(), pool, cfg, strings.Split(objectIDsParam, ","))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, geometry)
}

func parseBboxParams(r *http.Request) (onrequest.Bbox, error) {
	q := r.URL.Query()
	vals := make([]float64, 4)
	for i, key := range []string{"xmin", "ymin", "xmax", "ymax"} {
		v, err := strconv.ParseFloat(q.Get(key), 64)
		if err != nil {
			return onrequest.Bbox{}, fmt.Errorf("missing or invalid %s query param", key)
		}
		vals[i] = v
	}
	return onrequest.Bbox{Xmin: vals[0], Ymin: vals[1], Xmax: vals[2], Ymax: vals[3]}, nil
}
