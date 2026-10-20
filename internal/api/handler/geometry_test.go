package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGeometry_RejectsUnsupportedSRID pins that srid accepts only 4326, and that the
// refusal comes before any database lookup.
func TestGeometry_RejectsUnsupportedSRID(t *testing.T) {
	h := New(nil)
	for _, srid := range []string{"3857", "25832", "wgs84"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/geometry?country=germany&object_ids=a&srid="+srid, nil)
		rec := httptest.NewRecorder()
		h.Geometry(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), srid) {
			t.Errorf("srid=%s: status %d, body %s; want 400 naming the value", srid, rec.Code, rec.Body.String())
		}
	}
}
