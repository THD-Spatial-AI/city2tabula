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
	LodLevel  int       `json:"lod_level"`  // 2 for LOD2, 3 for LOD3, -1 for non-LOD tasks (e.g. schema setup)
	CreatedAt time.Time `json:"created_at"` // Creation timestamp
}

// NewTask creates a new Task instance.
// Set lodLevel to 2 or 3 for feature extraction tasks, -1 for everything else (schema setup, functions, etc.).
// We use -1 (not 0) because LOD0 is a real CityGML level and would be misleading.
func NewTask(taskType string, params Params, SQLFile string, priority int, lodLevel int) *Task {
	return &Task{
		TaskID:    uuid.New(),
		TaskType:  taskType,
		Params:    params,
		SQLFile:   SQLFile,
		Priority:  priority,
		LodLevel:  lodLevel,
		CreatedAt: time.Now(),
	}
}
