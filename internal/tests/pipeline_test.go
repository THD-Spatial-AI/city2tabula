package tests

import (
	"testing"
	"time"

	"City2TABULA/internal/process"

	"github.com/stretchr/testify/assert"
)

func TestNewPipeline(t *testing.T) {
	buildingIDs := []int64{1, 2, 3, 4, 5}
	jobs := []*process.Job{
		process.NewJob("JOB1", &process.Params{BuildingIDs: buildingIDs}, "job1.sql", 1),
		process.NewJob("JOB2", &process.Params{BuildingIDs: buildingIDs}, "job2.sql", 2),
	}

	pipeline := process.NewPipeline(buildingIDs, jobs)

	assert.NotNil(t, pipeline)
	assert.NotEqual(t, "", pipeline.PipelineID.String())
	assert.Equal(t, buildingIDs, pipeline.BuildingIDs)
	assert.Equal(t, jobs, pipeline.Jobs)
	assert.Len(t, pipeline.Jobs, 2)
	assert.NotEmpty(t, pipeline.CreatedAt)
}

func TestNewPipelineUniqueness(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	jobs := []*process.Job{process.NewJob("TEST", &process.Params{BuildingIDs: buildingIDs}, "test.sql", 1)}

	pipeline1 := process.NewPipeline(buildingIDs, jobs)
	pipeline2 := process.NewPipeline(buildingIDs, jobs)

	assert.NotEqual(t, pipeline1.PipelineID, pipeline2.PipelineID, "Each pipeline should have a unique ID")
}

func TestPipelineAddJob(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	initialJobs := []*process.Job{
		process.NewJob("INITIAL", &process.Params{BuildingIDs: buildingIDs}, "initial.sql", 1),
	}

	pipeline := process.NewPipeline(buildingIDs, initialJobs)
	assert.Len(t, pipeline.Jobs, 1)

	// Add a new job
	newJob := process.NewJob("ADDED", &process.Params{BuildingIDs: buildingIDs}, "added.sql", 2)
	pipeline.AddJob(newJob)

	assert.Len(t, pipeline.Jobs, 2)
	assert.Equal(t, "INITIAL", pipeline.Jobs[0].JobType)
	assert.Equal(t, "ADDED", pipeline.Jobs[1].JobType)
}

func TestPipelineAddMultipleJobs(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	pipeline := process.NewPipeline(buildingIDs, []*process.Job{})

	// Add multiple jobs
	for i := 0; i < 5; i++ {
		job := process.NewJob("JOB", &process.Params{BuildingIDs: buildingIDs}, "test.sql", i)
		pipeline.AddJob(job)
	}

	assert.Len(t, pipeline.Jobs, 5)

	// Verify order is maintained
	for i, job := range pipeline.Jobs {
		assert.Equal(t, i, job.Priority)
	}
}

func TestPipelineWithEmptyJobs(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	pipeline := process.NewPipeline(buildingIDs, []*process.Job{})

	assert.NotNil(t, pipeline)
	assert.Empty(t, pipeline.Jobs)
	assert.Equal(t, buildingIDs, pipeline.BuildingIDs)
}

func TestPipelineWithNilJobs(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	pipeline := process.NewPipeline(buildingIDs, nil)

	assert.NotNil(t, pipeline)
	assert.Nil(t, pipeline.Jobs)
	assert.Equal(t, buildingIDs, pipeline.BuildingIDs)
}

func TestPipelineWithEmptyBuildingIDs(t *testing.T) {
	buildingIDs := []int64{}
	jobs := []*process.Job{process.NewJob("TEST", &process.Params{}, "test.sql", 1)}
	pipeline := process.NewPipeline(buildingIDs, jobs)

	assert.NotNil(t, pipeline)
	assert.Empty(t, pipeline.BuildingIDs)
	assert.Len(t, pipeline.Jobs, 1)
}

func TestPipelineCreatedAtFormat(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}
	pipeline := process.NewPipeline(buildingIDs, []*process.Job{})

	// Test that CreatedAt is in RFC3339 format
	_, err := time.Parse(time.RFC3339, pipeline.CreatedAt)
	assert.NoError(t, err, "CreatedAt should be in RFC3339 format")
}

func TestPipelineCreationTimestamp(t *testing.T) {
	beforeCreation := time.Now()

	pipeline := process.NewPipeline([]int64{1, 2, 3}, []*process.Job{})

	afterCreation := time.Now()

	createdAt, err := time.Parse(time.RFC3339, pipeline.CreatedAt)
	assert.NoError(t, err)

	assert.True(t, createdAt.After(beforeCreation.Add(-time.Second)))
	assert.True(t, createdAt.Before(afterCreation.Add(time.Second)))
}

func TestPipelineEnqueuedAt(t *testing.T) {
	pipeline := process.NewPipeline([]int64{1, 2, 3}, []*process.Job{})

	// EnqueuedAt should be zero initially
	assert.True(t, pipeline.EnqueuedAt.IsZero())

	// Simulate enqueuing
	pipeline.EnqueuedAt = time.Now()
	assert.False(t, pipeline.EnqueuedAt.IsZero())
}

func TestPipelineJobOrder(t *testing.T) {
	buildingIDs := []int64{1, 2, 3}

	// Create jobs with different priorities
	job1 := process.NewJob("HIGH_PRIORITY", &process.Params{BuildingIDs: buildingIDs}, "high.sql", 1)
	job2 := process.NewJob("LOW_PRIORITY", &process.Params{BuildingIDs: buildingIDs}, "low.sql", 10)
	job3 := process.NewJob("MEDIUM_PRIORITY", &process.Params{BuildingIDs: buildingIDs}, "medium.sql", 5)

	jobs := []*process.Job{job1, job2, job3}
	pipeline := process.NewPipeline(buildingIDs, jobs)

	// Verify jobs are stored in the order they were provided (not sorted by priority)
	assert.Equal(t, "HIGH_PRIORITY", pipeline.Jobs[0].JobType)
	assert.Equal(t, "LOW_PRIORITY", pipeline.Jobs[1].JobType)
	assert.Equal(t, "MEDIUM_PRIORITY", pipeline.Jobs[2].JobType)
}

func TestPipelineWithLargeBuildingSet(t *testing.T) {
	// Test with a large set of building IDs
	buildingIDs := make([]int64, 10000)
	for i := range buildingIDs {
		buildingIDs[i] = int64(i + 1)
	}

	jobs := []*process.Job{process.NewJob("LARGE_SET", &process.Params{BuildingIDs: buildingIDs}, "large.sql", 1)}
	pipeline := process.NewPipeline(buildingIDs, jobs)

	assert.NotNil(t, pipeline)
	assert.Len(t, pipeline.BuildingIDs, 10000)
	assert.Equal(t, int64(1), pipeline.BuildingIDs[0])
	assert.Equal(t, int64(10000), pipeline.BuildingIDs[9999])
}

// Benchmark for pipeline creation
func BenchmarkNewPipeline(b *testing.B) {
	buildingIDs := []int64{1, 2, 3, 4, 5}
	jobs := []*process.Job{
		process.NewJob("JOB1", &process.Params{BuildingIDs: buildingIDs}, "job1.sql", 1),
		process.NewJob("JOB2", &process.Params{BuildingIDs: buildingIDs}, "job2.sql", 2),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		process.NewPipeline(buildingIDs, jobs)
	}
}

// Benchmark for adding jobs to pipeline
func BenchmarkPipelineAddJob(b *testing.B) {
	buildingIDs := []int64{1, 2, 3, 4, 5}
	pipeline := process.NewPipeline(buildingIDs, []*process.Job{})
	job := process.NewJob("BENCHMARK", &process.Params{BuildingIDs: buildingIDs}, "bench.sql", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipeline.AddJob(job)
	}
}
