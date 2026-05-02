package process

import (
	"fmt"
	"path/filepath"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// JobType represents the category of a job
type JobType string

const (
	LOD2               JobType = "lod2"
	LOD3               JobType = "lod3"
	Function           JobType = "function"
	MainTable          JobType = "main_table"
	Supplementary      JobType = "supplementary"
	SupplementaryTable JobType = "supplementary_table"
)

// BuildFeatureExtractionQueue creates a queue of jobs (one per batch)
// for both LOD2 and LOD3 building IDs.
func BuildFeatureExtractionQueue(
	config *config.Config,
	lod2Batches [][]int64,
	lod3Batches [][]int64,
) (*JobQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	jobQueue := NewJobQueue()

	// Build LOD2 jobs
	for _, batch := range lod2Batches {
		job := createJob(batch, sqlScripts.MainScripts, LOD2)
		jobQueue.Enqueue(job)
	}

	// Build LOD3 jobs
	for _, batch := range lod3Batches {
		job := createJob(batch, sqlScripts.MainScripts, LOD3)
		jobQueue.Enqueue(job)
	}

	return jobQueue, nil
}

// createJob is a helper function to create a job with tasks
func createJob(batch []int64, scripts []string, jobType JobType) *Job {
	params := Params{BuildingIDs: batch}
	job := NewJob(batch, nil)

	// Determine task name prefix based on job type
	var prefix string
	switch jobType {
	case LOD2:
		prefix = "LOD2"
	case LOD3:
		prefix = "LOD3"
	case Function:
		prefix = "FUNCTION"
	case MainTable:
		prefix = "MAIN_TABLE"
	case Supplementary:
		prefix = "SUPPLEMENTARY"
	case SupplementaryTable:
		prefix = "SUPPLEMENTARY_TABLE"
	}

	// Add tasks to job
	for i, file := range scripts {
		filename := filepath.Base(file)
		taskName := fmt.Sprintf("%s: %s", prefix, filename)
		job.AddTask(NewTask(taskName, params, file, i+1))
	}

	return job
}

// MainDBSetupJobQueue creates a queue with function scripts and main table scripts
func MainDBSetupJobQueue(config *config.Config) (*JobQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	jobQueue := NewJobQueue()

	// Add function scripts job
	functionJob := createJob([]int64{}, sqlScripts.FunctionScripts, Function)
	jobQueue.Enqueue(functionJob)

	// Add LOD2 main table scripts job
	lod2Job := createJob([]int64{}, sqlScripts.MainTableScripts, LOD2)
	jobQueue.Enqueue(lod2Job)

	// Add LOD3 main table scripts job
	lod3Job := createJob([]int64{}, sqlScripts.MainTableScripts, LOD3)
	jobQueue.Enqueue(lod3Job)

	return jobQueue, nil
}

// SupplementaryDBSetupJobQueue creates a queue with supplementary table scripts
func SupplementaryDBSetupJobQueue(config *config.Config) (*JobQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	jobQueue := NewJobQueue()

	job := createJob([]int64{}, sqlScripts.SupplementaryTableScripts, SupplementaryTable)
	jobQueue.Enqueue(job)

	return jobQueue, nil
}

// SupplementaryJobQueue creates a queue with supplementary processing scripts
func SupplementaryJobQueue(config *config.Config) (*JobQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	jobQueue := NewJobQueue()

	job := createJob([]int64{}, sqlScripts.SupplementaryScripts, Supplementary)
	jobQueue.Enqueue(job)

	return jobQueue, nil
}

func (j *Job) AddTask(task *Task) {
	j.Tasks = append(j.Tasks, task)
}
