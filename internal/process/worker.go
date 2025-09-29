package process

import (
	"City2TABULA/internal/config"
	"City2TABULA/internal/utils"
	"sync"

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
		utils.Info.Printf("[Worker %d] Starting pipeline %s", w.ID, pipeline.PipelineID)

		runner := NewRunner(config)
		if err := runner.RunPipeline(pipeline, conn, w.ID); err != nil {
			utils.Error.Printf("[Worker %d] Pipeline %s failed: %v", w.ID, pipeline.PipelineID, err)
			continue
		}

		utils.Info.Printf("[Worker %d] Pipeline %s completed successfully", w.ID, pipeline.PipelineID)
	}
}
