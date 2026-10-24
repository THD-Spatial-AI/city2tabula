package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSurfaces_RejectsBadRequests pins the 400s, all decided before any
// database lookup.
func TestSurfaces_RejectsBadRequests(t *testing.T) {
	h := New(nil)
	const bbox = "&xmin=12.9&ymin=48.8&xmax=13.0&ymax=48.9"
	tests := []struct{ name, query, wantErr string }{
		{"no country", "?" + bbox[1:], "country"},
		{"no bbox", "?country=germany", "xmin"},
		{"bbox value not a number", "?country=germany&xmin=a&ymin=48.8&xmax=13&ymax=48.9", "xmin"},
		{"cursor not one the API returned", "?country=germany" + bbox + "&cursor=%25%25", "cursor"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.Surfaces(rec, httptest.NewRequest(http.MethodGet, "/api/v1/surfaces"+tc.query, nil))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.wantErr) {
				t.Errorf("status %d, body %s; want 400 mentioning %q", rec.Code, rec.Body.String(), tc.wantErr)
			}
		})
	}
}
