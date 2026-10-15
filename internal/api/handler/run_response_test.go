package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/api/server"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// runJSON returns run as the status endpoint serialises it.
func runJSON(t *testing.T, run *server.Run) map[string]any {
	t.Helper()
	data, err := json.Marshal(toRunResponse(run))
	if err != nil {
		t.Fatalf("marshal run response: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal run response %s: %v", data, err)
	}
	return got
}

func TestToRunResponse_PendingRunHasNoStage(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	got := runJSON(t, &server.Run{ID: "r1", Status: server.StatusPending, CreatedAt: now, UpdatedAt: now})

	for _, key := range []string{"stage", "stage_label", "stage_index", "stage_count", "started_at"} {
		if v, ok := got[key]; ok {
			t.Errorf("pending run response has %s = %v, want it omitted", key, v)
		}
	}
	if got["created_at"] != "2026-10-15T12:00:00Z" || got["updated_at"] != "2026-10-15T12:00:00Z" {
		t.Errorf("timestamps = %v / %v, want RFC 3339 UTC", got["created_at"], got["updated_at"])
	}
}

func TestToRunResponse_RunningRunReportsStep(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	got := runJSON(t, &server.Run{
		ID: "r1", Status: server.StatusRunning, Stage: onrequest.Stages[1],
		CreatedAt: now, StartedAt: now.Add(time.Second), UpdatedAt: now.Add(2 * time.Second),
	})

	want := map[string]any{
		"stage":       "extract_features",
		"stage_label": "Extracting building features",
		"stage_index": float64(2),
		"stage_count": float64(3),
		"started_at":  "2026-10-15T12:00:01Z",
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %v, want %v", key, got[key], w)
		}
	}
}
