package server

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
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
