package process

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	ID int
}

func NewWorker(id int) *Worker {
	return &Worker{ID: id}
}

// Start drains jobChan until it's closed, running each job through a Runner.
// A failed job is logged and skipped rather than stopping the worker — one
// bad batch shouldn't block every other batch — but it increments failures
// so RunJobQueue can still report the run as failed overall.
func (w *Worker) Start(jobChan <-chan *Job, conn *pgxpool.Pool, wg *sync.WaitGroup, config *config.Config, failures *atomic.Int64) {
	defer wg.Done()

	for job := range jobChan {
		runner := NewRunner(config)
		if err := runner.RunJob(job, conn, w.ID); err != nil {
			utils.Error.Printf("[Worker %d] Job failed: %v", w.ID, err)
			failures.Add(1)
			continue
		}
	}
}

// RunJobQueue drains a JobQueue into a channel and processes it with a pool of
// workers (count from cfg.Batch.Threads). Returns an aggregate error if any
// job failed. See docs/code/job-queue.md for the full pipeline.
func RunJobQueue(queue *JobQueue, conn *pgxpool.Pool, cfg *config.Config) error {
	if queue.IsEmpty() {
		return nil
	}

	jobChan := queue.ToChannel()

	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 1; i <= cfg.Batch.Threads; i++ {
		wg.Add(1)
		go NewWorker(i).Start(jobChan, conn, &wg, cfg, &failures)
	}
	wg.Wait()

	if n := failures.Load(); n > 0 {
		return fmt.Errorf("%d job(s) failed, see logs above for details", n)
	}
	return nil
}
