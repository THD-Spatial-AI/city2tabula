// Package handler implements the on-request HTTP wrapper's handlers: trigger a
// City2TABULA run for a country/bbox, poll its status, and query already-linked
// 3D building data. See internal/api/server for the state these operate on.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/db"
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

// coverageResponse answers GET /api/v1/coverage. Configured is false when the
// country has no dataset at all, which Count alone cannot express: a count of
// zero otherwise means the dataset exists and nothing in the bbox is linked.
// The two lead a caller to different actions, so they stay separate fields.
type coverageResponse struct {
	Count      int  `json:"count"`
	Configured bool `json:"configured"`
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
	if errors.Is(err, db.ErrCountryNotConfigured) {
		writeJSON(w, http.StatusOK, coverageResponse{Count: 0, Configured: false})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	count, err := onrequest.CountBuildingLink(r.Context(), pool, cfg, bbox)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, coverageResponse{Count: count, Configured: true})
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
	if errors.Is(err, db.ErrCountryNotConfigured) {
		writeJSON(w, http.StatusOK, []onrequest.Building{})
		return
	}
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
//
// include=surfaces adds a building's individual envelope surface polygons, for
// at most maxSurfaceBuildings object_ids. Callers place a whole area on a map
// with the footprints, which are unrestricted, and ask for surfaces only for
// the buildings being shown in 3D.
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
	objectIDs := strings.Split(objectIDsParam, ",")
	includeSurfaces := r.URL.Query().Get("include") == "surfaces"

	// Validated before the pool lookup: the request is malformed whatever the
	// country turns out to be, and an unconfigured one answers 200 with an empty
	// result, which would swallow the refusal.
	if n := countNonEmpty(objectIDs); includeSurfaces && n > maxSurfaceBuildings {
		writeError(w, http.StatusBadRequest, fmt.Sprintf(
			"include=surfaces accepts at most %d object_id%s per request, but %d were given; split it into smaller requests",
			maxSurfaceBuildings, pluralS(maxSurfaceBuildings), n,
		))
		return
	}

	cfg, pool, err := h.srv.PoolFor(country)
	if errors.Is(err, db.ErrCountryNotConfigured) {
		writeJSON(w, http.StatusOK, []onrequest.BuildingGeometry{})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	geometry, err := onrequest.BuildingGeometryByObjectIDs(
		r.Context(), pool, cfg, objectIDs, includeSurfaces,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, geometry)
}

// maxSurfaceBuildings caps how many buildings one include=surfaces request may
// ask for. Deliberately low while the path is proven in production, and meant
// to be raised rather than treated as a permanent property of the API.
//
// Raising it does not make the bound safe by itself: payload size tracks the
// number of faces, not the number of buildings, and a building carries anywhere
// from a handful to over two hundred. A batch sized for median buildings still
// fails on a batch of large ones, so a higher limit wants a surface-count bound
// behind it rather than a bigger number here.
//
// Callers may also cap the response size they will accept, rejecting a whole
// response rather than truncating it. Nothing here can see that limit, so
// raising this one is a change to the largest response this endpoint can
// produce, not only to how many buildings a caller may name.
const maxSurfaceBuildings = 1

// pluralS keeps the limit message grammatical whatever maxSurfaceBuildings is
// set to.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// countNonEmpty counts the ids that carry a value, so a trailing or doubled
// comma does not read as an extra building.
func countNonEmpty(ids []string) int {
	n := 0
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			n++
		}
	}
	return n
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
