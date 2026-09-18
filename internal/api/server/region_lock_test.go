package server

import (
	"testing"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/onrequest"
)

// acquired reports whether acquireRegion returned within d, so a test can tell
// "proceeded" from "blocked" without depending on timing beyond that.
func acquired(s *Server, dbName string, bbox onrequest.Bbox, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.acquireRegion(dbName, bbox)
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestAcquireRegion_DisjointRegionsRunInParallel(t *testing.T) {
	s := New(config.Config{})
	first := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}
	second := onrequest.Bbox{Xmin: 5, Ymin: 5, Xmax: 6, Ymax: 6}

	s.acquireRegion("c2t_de", first)
	if !acquired(s, "c2t_de", second, time.Second) {
		t.Fatal("a disjoint region in the same database blocked; it should run in parallel")
	}
}

func TestAcquireRegion_SameBboxDifferentDatabasesRunInParallel(t *testing.T) {
	s := New(config.Config{})
	bbox := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}

	s.acquireRegion("c2t_de", bbox)
	if !acquired(s, "c2t_nl", bbox, time.Second) {
		t.Fatal("the same bbox in another database blocked; different databases share no rows")
	}
}

func TestAcquireRegion_OverlappingRegionsSerialise(t *testing.T) {
	s := New(config.Config{})
	first := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 10, Ymax: 10}
	overlapping := onrequest.Bbox{Xmin: 5, Ymin: 5, Xmax: 15, Ymax: 15}

	s.acquireRegion("c2t_de", first)

	// One waiter throughout: a second attempt would queue behind the first
	// once that one is woken, and report a block that is not the one under test.
	waiting := make(chan struct{})
	go func() {
		s.acquireRegion("c2t_de", overlapping)
		close(waiting)
	}()

	select {
	case <-waiting:
		t.Fatal("an overlapping region proceeded; the two could process the same building at once")
	case <-time.After(200 * time.Millisecond):
	}

	s.releaseRegion("c2t_de", first, "some-key")

	select {
	case <-waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting region was not woken after the overlapping run released")
	}
}

func TestReleaseRegion_ClearsTheDedupKey(t *testing.T) {
	s := New(config.Config{})
	bbox := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}
	key := runKey("c2t_de", bbox, "intersects")

	s.inflight[key] = "run-1"
	s.acquireRegion("c2t_de", bbox)
	s.releaseRegion("c2t_de", bbox, key)

	if _, still := s.inflight[key]; still {
		t.Error("the dedup key survived the run; an identical later request would join a finished run")
	}
	if len(s.active) != 0 {
		t.Errorf("active holds %d entries after release, want 0", len(s.active))
	}
}

func TestRunKey_DistinguishesWhatMatters(t *testing.T) {
	bbox := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}
	other := onrequest.Bbox{Xmin: 0, Ymin: 0, Xmax: 2, Ymax: 1}

	base := runKey("c2t_de", bbox, "intersects")
	if base != runKey("c2t_de", bbox, "intersects") {
		t.Error("the same request produced two keys; identical requests would never join")
	}
	for name, got := range map[string]string{
		"database":  runKey("c2t_nl", bbox, "intersects"),
		"bbox":      runKey("c2t_de", other, "intersects"),
		"bbox mode": runKey("c2t_de", bbox, "contains"),
	} {
		if got == base {
			t.Errorf("a differing %s produced the same key; unrelated requests would be merged", name)
		}
	}
}
