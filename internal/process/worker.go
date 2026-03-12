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

func (w *Worker) Start(pipelineChan <-chan *Pipeline, conn *pgxpool.Pool, wg *sync.WaitGroup, config *config.Config) {
	defer wg.Done()

	for pipeline := range pipelineChan {
		runner := NewRunner(config)
		if err := runner.RunPipeline(pipeline, conn, w.ID); err != nil {
			utils.Error.Printf("[Worker %d] Pipeline failed: %v", w.ID, err)
			continue
		}
	}
}

// RunPipelineQueue drains a PipelineQueue into a channel and runs it with workers.
func RunPipelineQueue(queue *PipelineQueue, conn *pgxpool.Pool, cfg *config.Config) error {
	if queue.IsEmpty() {
		return nil
	}

	pipChan := make(chan *Pipeline, queue.Len())
	for !queue.IsEmpty() {
		if p := queue.Dequeue(); p != nil {
			pipChan <- p
		}
	}
	close(pipChan)

	var wg sync.WaitGroup
	for i := 1; i <= cfg.Batch.Threads; i++ {
		wg.Add(1)
		go NewWorker(i).Start(pipChan, conn, &wg, cfg)
	}
	wg.Wait()
	return nil
}
