package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// The handler serialises a run after GetRun releases the lock while the run
// goroutine keeps writing, so GetRun must hand out a copy.
func TestGetRun_ReturnsSnapshot(t *testing.T) {
	s := New(config.Config{})
	s.runs["r1"] = &Run{ID: "r1", Status: StatusPending}

	run, ok := s.GetRun("r1")
	if !ok {
		t.Fatal("GetRun(r1): not found")
	}
	s.setRunStatus("r1", StatusRunning, "")

	if run.Status != StatusPending {
		t.Errorf("snapshot status changed to %q after a later update, want %q", run.Status, StatusPending)
	}
}

func TestSetRunStage_RecordsStageAndStartTime(t *testing.T) {
	s := New(config.Config{})
	s.runs["r1"] = &Run{ID: "r1", Status: StatusPending}

	if run, _ := s.GetRun("r1"); !run.StartedAt.IsZero() || run.Stage != (onrequest.Stage{}) {
		t.Fatalf("pending run has StartedAt %v and stage %+v, want both zero", run.StartedAt, run.Stage)
	}

	s.setRunStatus("r1", StatusRunning, "")
	started, _ := s.GetRun("r1")
	s.setRunStage("r1", onrequest.Stages[1])
	s.setRunStatus("r1", StatusFailed, "boom")

	run, _ := s.GetRun("r1")
	if run.Stage != onrequest.Stages[1] {
		t.Errorf("failed run stage = %+v, want the stage it failed in, %+v", run.Stage, onrequest.Stages[1])
	}
	if run.StartedAt.IsZero() || !run.StartedAt.Equal(started.StartedAt) {
		t.Errorf("StartedAt = %v, want the time the run entered running, %v", run.StartedAt, started.StartedAt)
	}
}

// An unknown id is how GET /api/v1/runs/{id} tells a never-started run, or one
// lost to a restart, from a live one.
func TestGetRun_UnknownIDNotFound(t *testing.T) {
	s := New(config.Config{})

	if run, ok := s.GetRun("missing"); ok || run != nil {
		t.Errorf("GetRun(missing) = %+v, %v; want nil, false", run, ok)
	}
}

// TestExecuteRun_LogsLifecycleWithRunFields drives a run against a database that
// cannot be reached, so it fails in its first stage, and pins that every line it
// logs identifies the run and that the failure names the stage.
func TestExecuteRun_LogsLifecycleWithRunFields(t *testing.T) {
	orig := utils.Logger
	var buf bytes.Buffer
	utils.Logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() { utils.Logger = orig })

	s := New(config.Config{})
	s.runs["r1"] = &Run{ID: "r1", Status: StatusPending}
	cfg := config.Config{
		Country: "germany",
		DB:      &config.DBConfig{Host: "127.0.0.1", Port: "1", Name: "unreachable", User: "u", Password: "p", SSLMode: "disable"},
	}
	bbox := onrequest.Bbox{Xmin: 8.79, Ymin: 53.14, Xmax: 8.82, Ymax: 53.16}

	s.executeRun("r1", cfg, bbox, "intersects", "key")

	var msgs []string
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var line map[string]any
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, raw)
		}
		if line["run_id"] != "r1" || line["country"] != "germany" || line["bbox"] != bbox.String() {
			t.Errorf("line lacks the run's fields: %v", line)
		}
		msgs = append(msgs, line["msg"].(string))
		if line["msg"] == "run failed" {
			if line["level"] != "ERROR" || line["stage"] != "import" || line["error"] == "" {
				t.Errorf("failure line = %v, want ERROR in stage import with the error", line)
			}
		}
	}
	if want := []string{"run started", "run stage", "run failed"}; !slices.Equal(msgs, want) {
		t.Errorf("logged %v, want %v", msgs, want)
	}
	if run, _ := s.GetRun("r1"); run.Status != StatusFailed || run.Stage != onrequest.Stages[0] {
		t.Errorf("run = %+v, want failed in stage %s", run, onrequest.Stages[0].Name)
	}
}
