package process

import (
	"sync"
	"time"
)

// PipelineQueue represents a queue for processing pipelines consisting of multiple jobs in chronological order.
type PipelineQueue struct {
	pipelines []*Pipeline
	mutex     sync.RWMutex
}

// NewPipelineQueue initializes an empty queue
func NewPipelineQueue() *PipelineQueue {
	return &PipelineQueue{
		pipelines: make([]*Pipeline, 0),
	}
}

// Enqueue adds a pipeline to the queue
func (pq *PipelineQueue) Enqueue(pipeline *Pipeline) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	pipeline.EnqueuedAt = time.Now()
	pq.pipelines = append(pq.pipelines, pipeline)
}

// Dequeue removes and returns the first pipeline
func (pq *PipelineQueue) Dequeue() *Pipeline {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if len(pq.pipelines) == 0 {
		return nil
	}

	pipeline := pq.pipelines[0]
	pq.pipelines = pq.pipelines[1:]
	return pipeline
}

// Len returns the number of pipelines
func (pq *PipelineQueue) Len() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.pipelines)
}

// IsEmpty checks if the pipeline queue is empty
func (pq *PipelineQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// Peek returns the first pipeline without removing it
func (pq *PipelineQueue) Peek() *Pipeline {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	if len(pq.pipelines) == 0 {
		return nil
	}

	return pq.pipelines[0]
}

// Clear removes all pipelines from the queue
func (pq *PipelineQueue) Clear() {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	pq.pipelines = pq.pipelines[:0]
}
