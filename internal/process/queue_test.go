package process

// Use case: When a job is enqueued, then it should be added to the queue and retrievable in the correct order.
//
// TestJobQueue tests the functionality of the JobQueue struct, creating, including enqueuing, dequeuing, peeking, and checking if the queue is empty.
//
// Case 1:
// Given: A new JobQueue is created.
// When: The queue is initialized.
// Then: The queue should be empty.
//
// Case 2:
// Given: A job is enqueued into the JobQueue.
// When: The job is added to the queue.
// Then: The length of the queue should increase by one, and the job should be retrievable using Peek and Dequeue.
//
// Case 3:
// Given: Multiple jobs are enqueued into the JobQueue.
// When: Multiple jobs are added to the queue.
// Then: The jobs should be retrievable in the order they were enqueued (FIFO order).
//
// Case 4:
// Given: A job is dequeued from the JobQueue.
// When: The first job is removed from the queue.
// Then: The length of the queue should decrease by one, and the dequeued job should no longer be in the queue.

import (
	"testing"
)

func TestJobQueue_NewQueueEmpty(t *testing.T) {
	// When
	queue := NewJobQueue()

	// Then
	if !queue.IsEmpty() {
		t.Errorf("Expected new queue to be empty, but it was not")
	}

	if queue.Len() != 0 {
		t.Errorf("Expected new queue length to be 0, but got %d", queue.Len())
	}
}

func TestJobQueue_Enqueue(t *testing.T) {
	// Given
	queue := NewJobQueue()
	job := NewJob([]int64{1}, nil)

	// When
	queue.Enqueue(job)

	// Then
	if queue.Len() != 1 {
		t.Errorf("Expected queue length to be 1 after enqueue, but got %d", queue.Len())
	}
}

func TestJobQueue_Dequeue(t *testing.T) {
	// Given
	queue := NewJobQueue()
	job1 := NewJob([]int64{1}, nil)
	job2 := NewJob([]int64{2}, nil)
	queue.Enqueue(job1)
	queue.Enqueue(job2)

	// When
	dequeuedJob_1 := queue.Dequeue()

	// Then
	if dequeuedJob_1 != job1 {
		t.Errorf("Expected first dequeued job to be job1, but got a different job")
	}
	if queue.Len() != 1 {
		t.Errorf("Expected queue length to be 1 after dequeue, but got %d", queue.Len())
	}

	// When
	dequeuedJob_2 := queue.Dequeue()

	// Then
	if dequeuedJob_2 != job2 {
		t.Errorf("Expected dequeued job to be job2, but got a different job")
	}

	// When
	dequeuedJob_3 := queue.Dequeue()

	// Then
	if dequeuedJob_3 != nil {
		t.Errorf("Expected dequeued job to be nil when queue is empty, but got a job")
	}
}

func TestJobQueue_IsEmpty(t *testing.T) {
	t.Run("given new queue, when IsEmpty called, then returns true", func(t *testing.T) {
		// Given
		queue := NewJobQueue()

		// When
		result := queue.IsEmpty()

		// Then
		if !result {
			t.Errorf("Expected new queue to be empty, but it was not")
		}
	})

	t.Run("given queue with one job, when IsEmpty called, then returns false", func(t *testing.T) {
		// Given
		queue := NewJobQueue()
		job := NewJob([]int64{1}, nil)
		queue.Enqueue(job)

		// When
		result := queue.IsEmpty()

		// Then
		if result {
			t.Errorf("Expected queue to not be empty after enqueue, but it was")
		}
	})
}

func TestJobQueue_Peek(t *testing.T) {
	t.Run("given empty queue, when Peek called, then returns nil", func(t *testing.T) {
		// Given
		queue := NewJobQueue()

		// When
		result := queue.Peek()

		// Then
		if result != nil {
			t.Errorf("Expected Peek to return nil for empty queue, but got a job")
		}
	})

	t.Run("given queue with one job, when Peek called, then returns the job without removing it", func(t *testing.T) {
		// Given
		queue := NewJobQueue()
		job := NewJob([]int64{1}, nil)
		queue.Enqueue(job)

		// When
		result := queue.Peek()

		// Then
		if result != job {
			t.Errorf("Expected Peek to return the enqueued job, but got a different job")
		}
	})
}

func TestJobQueue_Clear(t *testing.T) {
	// Given
	queue := NewJobQueue()
	job := NewJob([]int64{1}, nil)
	queue.Enqueue(job)

	// When
	queue.Clear()

	// Then
	if !queue.IsEmpty() {
		t.Errorf("Expected queue to be empty after Clear, but it was not")
	}
	if queue.Len() != 0 {
		t.Errorf("Expected queue length to be 0 after Clear, but got %d", queue.Len())
	}
}
