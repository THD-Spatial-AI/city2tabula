package process

import "testing"

// TestRunJobQueue_EmptyQueueReturnsImmediately covers the early return before
// any worker goroutine is spawned - safe to call with a nil conn/cfg since
// IsEmpty() is checked before either is ever touched.
func TestRunJobQueue_EmptyQueueReturnsImmediately(t *testing.T) {
	queue := NewJobQueue()

	if err := RunJobQueue(queue, nil, nil); err != nil {
		t.Errorf("expected nil error for an empty queue, got %v", err)
	}
}
