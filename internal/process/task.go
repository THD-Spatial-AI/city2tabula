package process

import (
	"time"

	"github.com/google/uuid"
)

/////////////////////
// define the task //
/////////////////////

// All required parameters for any SQL task
type Params struct {
	BuildingIDs []int64 `json:"building_ids"`
}

// Task represents a single parameterised SQL script execution
type Task struct {
	TaskID    uuid.UUID `json:"task_id"`    // Unique identifier for the task
	TaskType  string    `json:"task_type"`  // e.g. "LOD2: 01_extract.sql"
	Params    Params    `json:"params"`     // Parameters for the task
	SQLFile   string    `json:"sql_file"`   // SQL file path
	Priority  int       `json:"priority"`   // Task priority (lower number = higher priority)
	CreatedAt time.Time `json:"created_at"` // Creation timestamp
}

// NewTask creates a new Task instance
func NewTask(taskType string, params Params, SQLFile string, priority int) *Task {
	return &Task{
		TaskID:    uuid.New(),
		TaskType:  taskType,
		Params:    params,
		SQLFile:   SQLFile,
		Priority:  priority,
		CreatedAt: time.Now(),
	}
}
