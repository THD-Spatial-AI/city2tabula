package tests

import (
	"testing"
	"time"

	"City2TABULA/internal/process"

	"github.com/stretchr/testify/assert"
)

func TestNewJob(t *testing.T) {
	buildingIDs := []int64{1, 2, 3, 4, 5}
	params := &process.Params{BuildingIDs: buildingIDs}
	jobType := "AGGREGATE_SURFACES"
	sqlFile := "test.sql"
	priority := 1

	job := process.NewJob(jobType, params, sqlFile, priority)

	assert.NotNil(t, job)
	assert.NotEqual(t, "", job.JobID.String())
	assert.Equal(t, jobType, job.JobType)
	assert.Equal(t, params, job.Params)
	assert.Equal(t, sqlFile, job.SQLFile)
	assert.Equal(t, priority, job.Priority)
	assert.WithinDuration(t, time.Now(), job.CreatedAt, time.Second)
}

func TestNewJobUniqueness(t *testing.T) {
	params := &process.Params{BuildingIDs: []int64{1, 2, 3}}

	job1 := process.NewJob("TEST_JOB", params, "test.sql", 1)
	job2 := process.NewJob("TEST_JOB", params, "test.sql", 1)

	assert.NotEqual(t, job1.JobID, job2.JobID, "Each job should have a unique ID")
}

func TestJobParams(t *testing.T) {
	buildingIDs := []int64{100, 200, 300}
	params := &process.Params{BuildingIDs: buildingIDs}

	assert.Equal(t, buildingIDs, params.BuildingIDs)
	assert.Len(t, params.BuildingIDs, 3)
	assert.Contains(t, params.BuildingIDs, int64(200))
}

func TestJobWithEmptyParams(t *testing.T) {
	params := &process.Params{BuildingIDs: []int64{}}
	job := process.NewJob("EMPTY_TEST", params, "empty.sql", 0)

	assert.NotNil(t, job)
	assert.NotNil(t, job.Params)
	assert.Empty(t, job.Params.BuildingIDs)
}

func TestJobWithNilParams(t *testing.T) {
	job := process.NewJob("NIL_TEST", nil, "nil.sql", 0)

	assert.NotNil(t, job)
	assert.Nil(t, job.Params)
}

func TestJobPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
	}{
		{"High priority", 1},
		{"Normal priority", 5},
		{"Low priority", 10},
		{"Zero priority", 0},
		{"Negative priority", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := process.NewJob("PRIORITY_TEST", &process.Params{}, "test.sql", tt.priority)
			assert.Equal(t, tt.priority, job.Priority)
		})
	}
}

func TestJobCreationTimestamp(t *testing.T) {
	beforeCreation := time.Now()
	time.Sleep(time.Millisecond) // Small delay to ensure different timestamps

	job := process.NewJob("TIMESTAMP_TEST", &process.Params{}, "test.sql", 1)

	time.Sleep(time.Millisecond) // Small delay to ensure different timestamps
	afterCreation := time.Now()

	assert.True(t, job.CreatedAt.After(beforeCreation))
	assert.True(t, job.CreatedAt.Before(afterCreation))
}

func TestJobSQLFileTypes(t *testing.T) {
	tests := []struct {
		name    string
		sqlFile string
	}{
		{"SQL file", "query.sql"},
		{"Empty file", ""},
		{"File with path", "/path/to/query.sql"},
		{"File with special chars", "query-2024_v1.sql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := process.NewJob("SQL_TEST", &process.Params{}, tt.sqlFile, 1)
			assert.Equal(t, tt.sqlFile, job.SQLFile)
		})
	}
}

func TestJobTypes(t *testing.T) {
	jobTypes := []string{
		"AGGREGATE_SURFACES",
		"CALC_VOLUME",
		"CALC_STOREYS",
		"PROCESS_BUILDINGS",
		"EXTRACT_FEATURES",
	}

	for _, jobType := range jobTypes {
		t.Run(jobType, func(t *testing.T) {
			job := process.NewJob(jobType, &process.Params{}, "test.sql", 1)
			assert.Equal(t, jobType, job.JobType)
		})
	}
}

// Benchmark for job creation
func BenchmarkNewJob(b *testing.B) {
	params := &process.Params{BuildingIDs: []int64{1, 2, 3, 4, 5}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		process.NewJob("BENCHMARK_JOB", params, "test.sql", 1)
	}
}

// Test for large building ID sets
func TestJobWithLargeBuildingSet(t *testing.T) {
	// Create a large set of building IDs
	buildingIDs := make([]int64, 10000)
	for i := range buildingIDs {
		buildingIDs[i] = int64(i + 1)
	}

	params := &process.Params{BuildingIDs: buildingIDs}
	job := process.NewJob("LARGE_SET_TEST", params, "large.sql", 1)

	assert.NotNil(t, job)
	assert.Len(t, job.Params.BuildingIDs, 10000)
	assert.Equal(t, int64(1), job.Params.BuildingIDs[0])
	assert.Equal(t, int64(10000), job.Params.BuildingIDs[9999])
}
