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
