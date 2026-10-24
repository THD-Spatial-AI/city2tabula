package handler

import (
	"errors"
	"net/http"

	"github.com/thd-spatial-ai/city2tabula/internal/db"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// surfacePageVertices bounds one page by the vertices of its surfaces. Encoded
// at 7 decimals a vertex measures about 52 bytes with its share of the surface
// wrapper (Bavarian and Dutch data alike), so a page is about 4 MB, which stays
// under typical client response caps (10 MiB, for example).
const surfacePageVertices = 75_000

// surfacesResponse is the body of GET /api/v1/surfaces. NextCursor is null on
// the last page.
type surfacesResponse struct {
	Buildings    []onrequest.BuildingSurfaces `json:"buildings"`
	Attributions []onrequest.DatasetCredit    `json:"attributions"`
	NextCursor   *string                      `json:"next_cursor"`
}

// Surfaces handles GET /api/v1/surfaces?country=..&xmin=..&ymin=..&xmax=..&ymax=..[&cursor=..]:
// the roof and wall surfaces of every building in a WGS84 bbox, for rendering,
// one size-bounded page at a time.
func (h *Handler) Surfaces(w http.ResponseWriter, r *http.Request) {
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
	after, err := onrequest.DecodeSurfaceCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg, pool, err := h.srv.PoolFor(country)
	if errors.Is(err, db.ErrCountryNotConfigured) {
		writeJSON(w, http.StatusOK, surfacesResponse{Buildings: []onrequest.BuildingSurfaces{}, Attributions: []onrequest.DatasetCredit{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := onrequest.SurfacesInBBox(r.Context(), pool, cfg, bbox, after, surfacePageVertices)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	credits, err := onrequest.CreditsFor(r.Context(), pool, cfg, onrequest.SurfaceDatasetIDs(page.Buildings))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := surfacesResponse{Buildings: page.Buildings, Attributions: credits}
	if resp.Buildings == nil {
		resp.Buildings = []onrequest.BuildingSurfaces{}
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	writeJSON(w, http.StatusOK, resp)
}
