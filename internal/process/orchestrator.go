package process

import (
	"fmt"
	"path/filepath"

	"github.com/THD-Spatial/City2TABULA/internal/config"
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

// createJob builds a Job from a slice of SQL script paths.
// Each script becomes one Task, named "<PREFIX>: <filename>", with priority matching its position.
// LodLevel is set on each task so the runner knows which LOD schema to query — no string parsing needed.
func createJob(batch []int64, scripts []string, jobType JobType) *Job {
	params := Params{BuildingIDs: batch}
	job := NewJob(batch, nil)

	var prefix string
	lodLevel := -1 // -1 = no LOD context; LOD0 is a real CityGML level so we don't use 0 as a sentinel
	switch jobType {
	case LOD2:
		prefix, lodLevel = "LOD2", 2
	case LOD3:
		prefix, lodLevel = "LOD3", 3
	case Function:
		prefix = "FUNCTION"
	case MainTable:
		prefix = "MAIN_TABLE"
	case Supplementary:
		prefix = "SUPPLEMENTARY"
	case SupplementaryTable:
		prefix = "SUPPLEMENTARY_TABLE"
	}

	for i, file := range scripts {
		filename := filepath.Base(file)
		taskName := fmt.Sprintf("%s: %s", prefix, filename)
		job.AddTask(NewTask(taskName, params, file, i+1, lodLevel))
	}

	return job
}

// loadScriptsAndQueue loads SQL scripts from disk and returns an empty queue ready to fill.
// All queue-builder functions use this to avoid repeating the same error-handling boilerplate.
func loadScriptsAndQueue(config *config.Config) (*config.SQLScripts, *JobQueue, error) {
	scripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}
	return scripts, NewJobQueue(), nil
}

// MainDBSetupJobQueue builds the queue for the initial city2tabula schema setup:
//  1. PostgreSQL functions (run once, no building IDs)
//  2. LOD2 main tables
//  3. LOD3 main tables
func MainDBSetupJobQueue(config *config.Config) (*JobQueue, error) {
	scripts, queue, err := loadScriptsAndQueue(config)
	if err != nil {
		return nil, err
	}

	queue.Enqueue(createJob([]int64{}, scripts.FunctionScripts, Function))
	queue.Enqueue(createJob([]int64{}, scripts.MainTableScripts, LOD2))
	queue.Enqueue(createJob([]int64{}, scripts.MainTableScripts, LOD3))

	return queue, nil
}

// SupplementaryDBSetupJobQueue builds the queue for the supplementary schema setup
// (tabula classification tables). Runs after MainDBSetupJobQueue.
func SupplementaryDBSetupJobQueue(config *config.Config) (*JobQueue, error) {
	scripts, queue, err := loadScriptsAndQueue(config)
	if err != nil {
		return nil, err
	}

	queue.Enqueue(createJob([]int64{}, scripts.SupplementaryTableScripts, SupplementaryTable))

	return queue, nil
}

// SupplementaryJobQueue builds the queue for running the supplementary processing scripts
// (e.g. TABULA attribute extraction). Runs after data has been imported.
func SupplementaryJobQueue(config *config.Config) (*JobQueue, error) {
	scripts, queue, err := loadScriptsAndQueue(config)
	if err != nil {
		return nil, err
	}

	queue.Enqueue(createJob([]int64{}, scripts.SupplementaryScripts, Supplementary))

	return queue, nil
}
