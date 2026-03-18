package process

import (
	"sync"

	"github.com/THD-Spatial/City2TABULA/internal/config"
	"github.com/THD-Spatial/City2TABULA/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	ID int
}

func NewWorker(id int) *Worker {
	return &Worker{ID: id}
}

func (w *Worker) Start(jobChan <-chan *Job, conn *pgxpool.Pool, wg *sync.WaitGroup, config *config.Config) {
	defer wg.Done()

	for job := range jobChan {
		runner := NewRunner(config)
		if err := runner.RunJob(job, conn, w.ID); err != nil {
			utils.Error.Printf("[Worker %d] Job failed: %v", w.ID, err)
			continue
		}
	}
}

// RunJobQueue drains a JobQueue into a channel and processes it with workers.
func RunJobQueue(queue *JobQueue, conn *pgxpool.Pool, cfg *config.Config) error {
	if queue.IsEmpty() {
		return nil
	}

	jobChan := make(chan *Job, queue.Len())
	for !queue.IsEmpty() {
		if j := queue.Dequeue(); j != nil {
			jobChan <- j
		}
	}
	close(jobChan)

	var wg sync.WaitGroup
	for i := 1; i <= cfg.Batch.Threads; i++ {
		wg.Add(1)
		go NewWorker(i).Start(jobChan, conn, &wg, cfg)
	}
	wg.Wait()
	return nil
}
