package process

import (
	"time"

	"github.com/google/uuid"
)

// ///////////////////////////////////////////////////////////////
// define which tasks have to be executed in a sequence per batch //
// ///////////////////////////////////////////////////////////////

// Job represents a batch of building IDs and the sequence of tasks to execute for them
type Job struct {
	JobID       uuid.UUID `json:"job_id"`
	BuildingIDs []int64   `json:"building_ids"`
	Tasks       []*Task   `json:"tasks"`
	EnqueuedAt  time.Time `json:"enqueued_at"` // Timestamp when the job was enqueued
	CreatedAt   time.Time `json:"created_at"`  // Creation timestamp
}

// NewJob creates a new Job instance
func NewJob(buildingIDs []int64, tasks []*Task) *Job {
	return &Job{
		JobID:       uuid.New(),
		BuildingIDs: buildingIDs,
		Tasks:       tasks,
		CreatedAt:   time.Now(),
	}
}
