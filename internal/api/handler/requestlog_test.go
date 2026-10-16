package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// captureLog points utils.Logger at a JSON buffer at DEBUG for the test and
// returns a func that decodes every line written so far.
func captureLog(t *testing.T) func() []map[string]any {
	t.Helper()
	orig := utils.Logger
	var buf bytes.Buffer
	utils.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() { utils.Logger = orig })
	return func() []map[string]any {
		var lines []map[string]any
		for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if raw == "" {
				continue
			}
			var line map[string]any
			if err := json.Unmarshal([]byte(raw), &line); err != nil {
				t.Fatalf("log line is not JSON: %v\n%s", err, raw)
			}
			lines = append(lines, line)
		}
		return lines
	}
}

// serve sends one request through LogRequests to a handler answering status.
func serve(path, requestID string, status int) *httptest.ResponseRecorder {
	h := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if requestID != "" {
		req.Header.Set(RequestIDHeader, requestID)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLogRequests_LogsOneLinePerRequest(t *testing.T) {
	lines := captureLog(t)

	rec := serve("/api/v1/coverage?country=germany&xmin=1", "", http.StatusOK)

	id := rec.Header().Get(RequestIDHeader)
	if id == "" {
		t.Fatal("response has no X-Request-ID")
	}
	got := lines()
	if len(got) != 1 {
		t.Fatalf("got %d log lines, want 1: %v", len(got), got)
	}
	want := map[string]any{
		"msg": "http request", "level": "INFO", "request_id": id, "method": "GET",
		"path": "/api/v1/coverage", "query": "country=germany&xmin=1", "status": float64(200),
	}
	for k, v := range want {
		if got[0][k] != v {
			t.Errorf("%s = %v, want %v", k, got[0][k], v)
		}
	}
	if _, ok := got[0]["duration_ms"]; !ok {
		t.Error("log line has no duration_ms")
	}
}

func TestLogRequests_RequestID(t *testing.T) {
	for _, tc := range []struct {
		name, sent string
		keep       bool
	}{
		{"caller id kept", "enerplanet-42.a_b", true},
		{"newline replaced", "abc\nlevel=ERROR", false},
		{"too long replaced", strings.Repeat("a", 129), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captureLog(t)
			got := serve("/api/v1/buildings", tc.sent, http.StatusOK).Header().Get(RequestIDHeader)
			if tc.keep && got != tc.sent {
				t.Errorf("X-Request-ID = %q, want the caller's %q", got, tc.sent)
			}
			if !tc.keep && (got == tc.sent || !validRequestID.MatchString(got)) {
				t.Errorf("X-Request-ID = %q, want a fresh valid id instead of %q", got, tc.sent)
			}
		})
	}
}

func TestLogRequests_LevelFollowsStatus(t *testing.T) {
	for _, tc := range []struct {
		path   string
		status int
		level  string
	}{
		{"/api/v1/runs/x", http.StatusInternalServerError, "ERROR"},
		{"/api/v1/runs/x", http.StatusNotFound, "WARN"},
		{"/api/v1/health", http.StatusOK, "DEBUG"},
		{"/api/v1/health", http.StatusServiceUnavailable, "ERROR"},
	} {
		lines := captureLog(t)
		serve(tc.path, "", tc.status)
		if got := lines(); len(got) != 1 || got[0]["level"] != tc.level {
			t.Errorf("%s answering %d: logged %v, want one line at %s", tc.path, tc.status, got, tc.level)
		}
	}
}
