package process

import (
	"sync"
	"time"
)

// JobQueue represents a queue of jobs to be processed by the worker pool.
type JobQueue struct {
	jobs  []*Job
	mutex sync.RWMutex
}

// NewJobQueue initializes an empty queue
func NewJobQueue() *JobQueue {
	return &JobQueue{
		jobs: make([]*Job, 0),
	}
}

// Enqueue adds a job to the queue
func (q *JobQueue) Enqueue(job *Job) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	job.EnqueuedAt = time.Now()
	q.jobs = append(q.jobs, job)
}

// Dequeue removes and returns the first job
func (q *JobQueue) Dequeue() *Job {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// Len returns the number of jobs in the queue
func (q *JobQueue) Len() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.jobs)
}

// IsEmpty checks if the job queue is empty
func (q *JobQueue) IsEmpty() bool {
	return q.Len() == 0
}

// Peek returns the first job without removing it
func (q *JobQueue) Peek() *Job {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	if len(q.jobs) == 0 {
		return nil
	}

	return q.jobs[0]
}

// Clear removes all jobs from the queue
func (q *JobQueue) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.jobs = q.jobs[:0]
}

// ToChannel drains all jobs into a buffered channel and closes it.
// Use this to hand off a queue to a worker pool — the channel is ready to range over immediately.
func (q *JobQueue) ToChannel() <-chan *Job {
	ch := make(chan *Job, q.Len())
	for !q.IsEmpty() {
		if j := q.Dequeue(); j != nil {
			ch <- j
		}
	}
	close(ch)
	return ch
}
