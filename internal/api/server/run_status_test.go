package server

import (
	"testing"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
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
