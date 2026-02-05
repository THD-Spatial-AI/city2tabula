package process

import (
	"City2TABULA/internal/config"
	"fmt"
	"path/filepath"
)

// PipelineType represents different types of pipeline operations
type PipelineType string

const (
	LOD2               PipelineType = "lod2"
	LOD3               PipelineType = "lod3"
	Function           PipelineType = "function"
	MainTable          PipelineType = "main_table"
	Supplementary      PipelineType = "supplementary"
	SupplementaryTable PipelineType = "supplementary_table"
)

// BuildFeatureExtractionQueue creates a queue of pipelines (one per batch)
// for both LOD2 and LOD3 building IDs.
func BuildFeatureExtractionQueue(
	config *config.Config,
	lod2Batches [][]int64,
	lod3Batches [][]int64,
) (*PipelineQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	// Build LOD2 pipelines
	for _, batch := range lod2Batches {
		pipeline := createPipeline(batch, sqlScripts.MainScripts, LOD2, 2)
		pipelineQueue.Enqueue(pipeline)
	}

	// Build LOD3 pipelines
	for _, batch := range lod3Batches {
		pipeline := createPipeline(batch, sqlScripts.MainScripts, LOD3, 3)
		pipelineQueue.Enqueue(pipeline)
	}

	return pipelineQueue, nil
}

// createPipeline is a helper function to create a pipeline with jobs
func createPipeline(batch []int64, scripts []string, pipelineType PipelineType, lod int) *Pipeline {
	params := Params{BuildingIDs: batch}
	pipeline := NewPipeline(batch, nil)

	// Determine job name prefix based on pipeline type
	var prefix string
	switch pipelineType {
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

	// Add jobs to pipeline
	for i, file := range scripts {
		filename := filepath.Base(file)
		jobName := fmt.Sprintf("%s: %s", prefix, filename)
		pipeline.AddJob(NewJob(jobName, params, file, i+1))
	}

	return pipeline
}

// MainDBSetupPipelineQueue creates a queue with function scripts and main table scripts
func MainDBSetupPipelineQueue(config *config.Config) (*PipelineQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	// Add function scripts pipeline
	functionPipeline := createPipeline([]int64{}, sqlScripts.FunctionScripts, Function, 0)
	pipelineQueue.Enqueue(functionPipeline)

	// Add LOD2 main table scripts pipeline
	lod2Pipeline := createPipeline([]int64{}, sqlScripts.MainTableScripts, LOD2, 2)
	pipelineQueue.Enqueue(lod2Pipeline)

	// Add LOD3 main table scripts pipeline
	lod3Pipeline := createPipeline([]int64{}, sqlScripts.MainTableScripts, LOD3, 3)
	pipelineQueue.Enqueue(lod3Pipeline)

	return pipelineQueue, nil
}

// SupplementaryDBSetupPipelineQueue creates a queue with supplementary table scripts
func SupplementaryDBSetupPipelineQueue(config *config.Config) (*PipelineQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	pipeline := createPipeline([]int64{}, sqlScripts.SupplementaryTableScripts, SupplementaryTable, 0)
	pipelineQueue.Enqueue(pipeline)

	return pipelineQueue, nil
}

// SupplementaryPipelineQueue creates a queue with supplementary processing scripts
func SupplementaryPipelineQueue(config *config.Config) (*PipelineQueue, error) {
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	pipeline := createPipeline([]int64{}, sqlScripts.SupplementaryScripts, Supplementary, 0)
	pipelineQueue.Enqueue(pipeline)

	return pipelineQueue, nil
}
