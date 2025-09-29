package process

import (
	"City2TABULA/internal/config"
	"fmt"
	"path/filepath"
)

// BuildFeatureExtractionQueue creates a queue of pipelines (one per batch)
// for both LOD2 and LOD3 building IDs.
func BuildFeatureExtractionQueue(
	config *config.Config,
	lod2Batches [][]int64,
	lod3Batches [][]int64,
) (*PipelineQueue, error) {
	// Load SQL scripts in correct order
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	// helper for creating pipelines
	createPipeline := func(batch []int64, lod int) {
		params := Params{
			BuildingIDs: batch,
		}

		pipeline := NewPipeline(batch, nil)

		// prefix job names with LOD number for clarity
		lodLabel := func(name string) string {
			return "LOD" + fmt.Sprint(lod) + " " + name
		}

		jobPriority := 1
		// Finally, add main script jobs in order (core feature extraction pipeline)
		for _, file := range sqlScripts.MainScripts {
			pipeline.AddJob(NewJob(lodLabel(file), &params, file, jobPriority))
			jobPriority++
		}

		// Enqueue the pipeline
		pipelineQueue.Enqueue(pipeline)
	}

	// Build pipelines for LOD2
	for _, batch := range lod2Batches {
		createPipeline(batch, 2)
	}

	// Build pipelines for LOD3
	for _, batch := range lod3Batches {
		createPipeline(batch, 3)
	}

	return pipelineQueue, nil
}

func DBSetupPipelineQueue(config *config.Config) (*PipelineQueue, error) {
	// Load SQL scripts in correct order
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	// Create a single pipeline for DB setup
	pipeline := NewPipeline([]int64{}, nil)

	params := Params{
		BuildingIDs: []int64{},
	}

	jobPriority := 1

	for _, file := range sqlScripts.TableScripts {
		pipeline.AddJob(NewJob("LOD2", &params, file, jobPriority))
		jobPriority++
	}

	for _, file := range sqlScripts.TableScripts {
		pipeline.AddJob(NewJob("LOD3", &params, file, jobPriority))
		jobPriority++
	}

	// Add main script jobs in order (core feature extraction pipeline)
	for _, file := range sqlScripts.FunctionScripts {
		filename := filepath.Base(file)
		pipeline.AddJob(NewJob("FUNCTION: "+filename, &params, file, jobPriority))
		jobPriority++
	}

	// Enqueue the pipeline
	pipelineQueue.Enqueue(pipeline)

	return pipelineQueue, nil
}

func SupplementaryPipelineQueue(config *config.Config) (*PipelineQueue, error) {
	// Load SQL scripts in correct order
	sqlScripts, err := config.LoadSQLScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to load SQL scripts: %w", err)
	}

	pipelineQueue := NewPipelineQueue()

	// Create a single pipeline for DB setup
	pipeline := NewPipeline([]int64{}, nil)

	params := Params{
		BuildingIDs: []int64{},
	}

	jobPriority := 1

	for _, file := range sqlScripts.SupplementaryScripts {
		filename := filepath.Base(file)
		pipeline.AddJob(NewJob("SUPPLEMENTARY: "+filename, &params, file, jobPriority))
		jobPriority++
	}

	// Enqueue the pipeline
	pipelineQueue.Enqueue(pipeline)

	return pipelineQueue, nil
}
