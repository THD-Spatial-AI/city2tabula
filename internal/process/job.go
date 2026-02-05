package process

import (
	"time"

	"github.com/google/uuid"
)

/////////////////////
// define the task //
/////////////////////

// All required parameters for any SQL job
type Params struct {
	BuildingIDs []int64 `json:"building_ids"`
}

// Job represents a database job with its parameters and SQL file
type Job struct {
	JobID     uuid.UUID `json:"job_id"`     // Unique identifier for the job
	JobType   string    `json:"job_type"`   // e.g. "AGGREGATE_SURFACES"
	Params    Params    `json:"params"`     // Parameters for the job
	SQLFile   string    `json:"sql_file"`   // SQL file information
	Priority  int       `json:"priority"`   // Job priority (lower number = higher priority)
	CreatedAt time.Time `json:"created_at"` // Creation timestamp
}

// NewJob creates a new Job instance
func NewJob(jobType string, params Params, SQLFile string, Priority int) *Job {
	return &Job{
		JobID:     uuid.New(),
		JobType:   jobType,
		Params:    params,
		SQLFile:   SQLFile,
		Priority:  Priority,
		CreatedAt: time.Now(),
	}
}
