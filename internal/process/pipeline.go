package process

import (
	"time"

	"github.com/google/uuid"
)

// /////////////////////////////////////////////////////
// define which jobs has to be executed in a sequence //
// /////////////////////////////////////////////////////

// Pipeline represents a sequence of jobs to be executed
type Pipeline struct {
	PipelineID  uuid.UUID `json:"pipeline_id"`
	BuildingIDs []int64   `json:"building_ids"`
	Jobs        []*Job    `json:"jobs"`
	EnqueuedAt  time.Time `json:"enqueued_at"` // Timestamp when the pipeline was enqueued
	CreatedAt   string    `json:"created_at"`  // Creation timestamp
}

// NewPipeline creates a new Pipeline instance
func NewPipeline(buildingIDs []int64, jobs []*Job) *Pipeline {
	return &Pipeline{
		PipelineID:  uuid.New(),
		BuildingIDs: buildingIDs,
		Jobs:        jobs,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
}

func (p *Pipeline) AddJob(job *Job) {
	p.Jobs = append(p.Jobs, job)
}
