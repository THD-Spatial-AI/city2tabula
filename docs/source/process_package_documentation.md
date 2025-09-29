# Process Package Documentation

The `process` package provides a job-based pipeline system for executing database operations and feature extraction workflows in the City2TABULA application. This package replaces the complexity of multiple separate database modules with a unified, queue-based processing system.

## Package Overview

```
internal/process/
├── job.go          # Core job definitions and structures
├── pipeline.go     # Pipeline orchestration
├── queue.go        # Thread-safe queue implementations
├── runner.go       # Job execution engine
├── worker.go       # Concurrent worker implementation
├── orchestrator.go # Pipeline building and orchestration
├── print.go        # Debugging and monitoring utilities
└── database.go     # Database setup and schema creation
```

## Core Architecture

The process package implements a **Producer-Consumer** pattern with the following components:

1. **Jobs** - Individual SQL operations with parameters
2. **Pipelines** - Sequences of related jobs
3. **Queues** - Thread-safe storage for pipelines/jobs
4. **Workers** - Concurrent processors that execute pipelines
5. **Runners** - Execute individual jobs with parameter substitution

---

## Module Documentation

### 1. job.go

Core data structures for job management and SQL execution.

#### Structs

##### `Params`
```go
type Params struct {
    BuildingIDs []int64        `json:"building_ids"`
    Tables      *utils.Tables  `json:"table_names"`
    Schemas     *utils.Schemas `json:"schema_names"`
}
```

**Purpose**: Contains all required parameters for any SQL job.

**Fields**:
- `BuildingIDs`: List of building feature IDs to process
- `Tables`: Reference to table configuration from config
- `Schemas`: Reference to schema configuration from config

**Usage**: Passed to jobs to provide dynamic parameter replacement in SQL templates.

##### `SQLFile`
```go
type SQLFile struct {
    DisplayName    string        `json:"display_name"`
    Path           utils.SQLPath `json:"path"`
    RequiredParams []string      `json:"required_params"`
}
```

**Purpose**: Encapsulates SQL file information and parameter requirements.

**Fields**:
- `DisplayName`: Human-readable name for logging and debugging
- `Path`: File system path to the SQL template file
- `RequiredParams`: List of parameter keys that must be replaced in the SQL template

**Example**:
```go
sqlFile := &SQLFile{
    DisplayName:    "Extract Child Features",
    Path:           "/path/to/01_get_child_feat.sql",
    RequiredParams: []string{"lod_schema", "city2tabula_schema", "building_ids"},
}
```

##### `Job`
```go
type Job struct {
    JobID     uuid.UUID `json:"job_id"`
    JobType   string    `json:"job_type"`
    Params    *Params   `json:"params"`
    SQLFile   *SQLFile  `json:"sql_file"`
    Priority  int       `json:"priority"`
    CreatedAt time.Time `json:"created_at"`
}
```

**Purpose**: Represents a single database operation with all necessary context.

**Fields**:
- `JobID`: Unique identifier for tracking and debugging
- `JobType`: Descriptive type (e.g., "LOD2 Child Feature Extraction")
- `Params`: Parameters for SQL template replacement
- `SQLFile`: SQL file information and requirements
- `Priority`: Execution order (lower numbers execute first)
- `CreatedAt`: Timestamp for audit and debugging

#### Functions

##### `NewJob(jobType string, params *Params, SQLFile *SQLFile, Priority int) *Job`

**Purpose**: Factory function to create new Job instances with auto-generated UUID and timestamp.

**Parameters**:
- `jobType`: Descriptive name for the job
- `params`: Parameter set for SQL template replacement
- `SQLFile`: SQL file configuration
- `Priority`: Execution priority

**Returns**: Pointer to initialized Job instance

**Example**:
```go
params := &Params{
    BuildingIDs: []int64{1001, 1002, 1003},
    Tables:      &config.Tables,
    Schemas:     &config.Schemas,
}

sqlFile := &SQLFile{
    DisplayName:    "LOD2 Child Feature Extraction",
    Path:           config.DB.SQLChildFeatureFile,
    RequiredParams: []string{"lod_schema", "city2tabula_schema", "building_ids"},
}

job := NewJob("LOD2_CHILD_FEATURES", params, sqlFile, 1)
```

---

### 2. pipeline.go

Manages sequences of related jobs that must be executed together.

#### Structs

##### `Pipeline`
```go
type Pipeline struct {
    PipelineID  uuid.UUID `json:"pipeline_id"`
    BuildingIDs []int64   `json:"building_ids"`
    Jobs        []*Job    `json:"jobs"`
    EnqueuedAt  time.Time `json:"enqueued_at"`
    CreatedAt   string    `json:"created_at"`
}
```

**Purpose**: Groups related jobs that process the same set of building IDs.

**Fields**:
- `PipelineID`: Unique identifier for the pipeline
- `BuildingIDs`: Building IDs that all jobs in this pipeline will process
- `Jobs`: Ordered list of jobs to execute
- `EnqueuedAt`: Timestamp when pipeline was added to queue
- `CreatedAt`: Pipeline creation timestamp (RFC3339 format)

**Usage**: Ensures that all feature extraction steps for a batch of buildings are executed together and in order.

#### Functions

##### `NewPipeline(buildingIDs []int64, jobs []*Job) *Pipeline`

**Purpose**: Factory function to create new Pipeline instances.

**Parameters**:
- `buildingIDs`: List of building IDs this pipeline will process
- `jobs`: Initial list of jobs (can be nil, jobs added later with AddJob)

**Returns**: Pointer to initialized Pipeline instance

##### `(p *Pipeline) AddJob(job *Job)`

**Purpose**: Adds a job to the pipeline's job list.

**Parameters**:
- `job`: Job to add to the pipeline

**Example**:
```go
pipeline := NewPipeline([]int64{1001, 1002}, nil)
pipeline.AddJob(childFeatureJob)
pipeline.AddJob(geometryDumpJob)
pipeline.AddJob(attributeCalcJob)
```

---

### 3. queue.go

Thread-safe queue implementations for managing pipelines and jobs.

#### Structs

##### `PipelineQueue`
```go
type PipelineQueue struct {
    pipelines []*Pipeline
    mutex     sync.RWMutex
}
```

**Purpose**: Thread-safe FIFO queue for managing pipelines awaiting execution.

**Fields**:
- `pipelines`: Internal slice storing pipeline pointers
- `mutex`: Read-write mutex for thread safety

##### `JobQueue`
```go
type JobQueue struct {
    jobs  []*Job
    mutex sync.RWMutex
}
```

**Purpose**: Thread-safe FIFO queue for managing individual jobs.

**Fields**:
- `jobs`: Internal slice storing job pointers
- `mutex`: Read-write mutex for thread safety

#### PipelineQueue Functions

##### `NewPipelineQueue() *PipelineQueue`
**Purpose**: Creates empty pipeline queue
**Returns**: Initialized PipelineQueue instance

##### `(pq *PipelineQueue) Enqueue(pipeline *Pipeline)`
**Purpose**: Adds pipeline to end of queue
**Parameters**: `pipeline` - Pipeline to add
**Thread Safety**: Mutex-protected

##### `(pq *PipelineQueue) Dequeue() *Pipeline`
**Purpose**: Removes and returns first pipeline from queue
**Returns**: Pipeline pointer or nil if queue empty
**Thread Safety**: Mutex-protected

##### `(pq *PipelineQueue) Len() int`
**Purpose**: Returns current queue length
**Returns**: Number of pipelines in queue
**Thread Safety**: Read-lock protected

##### `(pq *PipelineQueue) IsEmpty() bool`
**Purpose**: Checks if queue is empty
**Returns**: true if no pipelines in queue

##### `(pq *PipelineQueue) Peek() *Pipeline`
**Purpose**: Returns first pipeline without removing it
**Returns**: Pipeline pointer or nil if queue empty

##### `(pq *PipelineQueue) Clear()`
**Purpose**: Removes all pipelines from queue
**Thread Safety**: Mutex-protected

##### `(pq *PipelineQueue) GetPendingPipelines() []*Pipeline`
**Purpose**: Returns copy of all pending pipelines
**Returns**: Slice copy of all pipelines
**Thread Safety**: Read-lock protected

#### JobQueue Functions

JobQueue provides identical methods to PipelineQueue but for individual jobs:
- `NewJobQueue()`, `Enqueue()`, `Dequeue()`, `Len()`, `IsEmpty()`, `Peek()`, `Clear()`, `GetPendingJobs()`

**Example Usage**:
```go
queue := NewPipelineQueue()
queue.Enqueue(pipeline1)
queue.Enqueue(pipeline2)

for !queue.IsEmpty() {
    pipeline := queue.Dequeue()
    // Process pipeline
}
```

---

### 4. runner.go

Executes jobs with SQL parameter substitution and database operations.

#### Structs

##### `Runner`
```go
type Runner struct {
    job      *Job
    pipeline *Pipeline
}
```

**Purpose**: Executes individual jobs and pipelines with parameter substitution.

**Fields**:
- `job`: Current job being executed (can be nil)
- `pipeline`: Current pipeline being executed (can be nil)

#### Functions

##### `NewRunner(job *Job, pipeline *Pipeline) *Runner`
**Purpose**: Creates new Runner instance
**Parameters**: `job` and `pipeline` can be nil depending on use case
**Returns**: Initialized Runner instance

##### `(r *Runner) RunPipeline(pipeline *Pipeline, conn *pgxpool.Pool) error`
**Purpose**: Executes all jobs in a pipeline in priority order

**Parameters**:
- `pipeline`: Pipeline to execute
- `conn`: Database connection pool

**Process**:
1. Sorts jobs by priority (ascending)
2. Executes each job sequentially
3. Returns error if any job fails

**Returns**: Error if any job fails, nil on success

##### `(r *Runner) RunJob(job *Job, conn *pgxpool.Pool) error`
**Purpose**: Executes a single job with parameter substitution

**Parameters**:
- `job`: Job to execute
- `conn`: Database connection pool

**Process**:
1. Reads SQL file from job.SQLFile.Path
2. Determines execution method (schema vs LOD-specific)
3. Performs parameter substitution
4. Executes SQL against database

**Returns**: Error if execution fails

##### `(r *Runner) getSQLScript(path string) (string, error)`
**Purpose**: Reads SQL file content from filesystem
**Parameters**: `path` - File system path to SQL file
**Returns**: SQL content string and error

##### `(r *Runner) replaceParameters(sqlScript string, params map[string]any) (string, error)`
**Purpose**: Replaces `{parameter_name}` placeholders in SQL with actual values

**Parameters**:
- `sqlScript`: SQL template with placeholders
- `params`: Key-value map of parameters

**Example**:
```sql
-- Before replacement
SELECT * FROM {city2tabula_schema}.{lod_schema}_child_feature 
WHERE building_feature_id IN {building_ids}

-- After replacement
SELECT * FROM training.lod2_child_feature 
WHERE building_feature_id IN (1001,1002,1003)
```

##### `(r *Runner) executeSQLScript(sqlScript string, conn *pgxpool.Pool, job *Job, lod int) error`
**Purpose**: Executes LOD-specific jobs with full parameter substitution

**Parameter Mapping**:
- `lod_schema` → "lod2" or "lod3" based on LOD level
- `city2tabula_schema` → config.DB.Schemas.Training
- `tabula_schema` → config.DB.Schemas.Tabula
- `lod_level` → 2 or 3
- `building_ids` → "(1001,1002,1003)" formatted list

##### `(r *Runner) executeSchemaSQL(sqlScript string, conn *pgxpool.Pool, job *Job) error`
**Purpose**: Executes schema creation jobs with simplified parameters

**Parameter Mapping**:
- `city2tabula_schema` → config.DB.Schemas.Training
- `tabula_schema` → config.DB.Schemas.Tabula

---

### 5. worker.go

Concurrent worker implementation for processing pipelines.

#### Structs

##### `Worker`
```go
type Worker struct {
    ID int
}
```

**Purpose**: Represents a concurrent worker that processes pipelines from a channel.

**Fields**:
- `ID`: Unique identifier for the worker (used in logging)

#### Functions

##### `NewWorker(id int) *Worker`
**Purpose**: Creates new Worker instance
**Parameters**: `id` - Unique worker identifier
**Returns**: Initialized Worker instance

##### `(w *Worker) Start(pipelineChan <-chan *Pipeline, conn *pgxpool.Pool, wg *sync.WaitGroup)`
**Purpose**: Starts worker to process pipelines from channel

**Parameters**:
- `pipelineChan`: Read-only channel of pipelines to process
- `conn`: Database connection pool
- `wg`: WaitGroup for coordinating worker completion

**Process**:
1. Receives pipelines from channel until channel closes
2. Creates runner for each pipeline
3. Executes pipeline using runner
4. Logs success/failure
5. Calls `wg.Done()` when channel closes

**Usage Example**:
```go
numWorkers := 4
var wg sync.WaitGroup

pipelineChan := make(chan *Pipeline, queue.Len())

// Start workers
for i := 1; i <= numWorkers; i++ {
    wg.Add(1)
    worker := NewWorker(i)
    go worker.Start(pipelineChan, pool, &wg)
}

// Enqueue pipelines
for !queue.IsEmpty() {
    pipelineChan <- queue.Dequeue()
}
close(pipelineChan)

// Wait for completion
wg.Wait()
```

---

### 6. orchestrator.go

High-level pipeline building and workflow orchestration.

#### Functions

##### `BuildFeatureExtractionQueue(config *utils.Config, lod2Batches [][]int64, lod3Batches [][]int64) *PipelineQueue`

**Purpose**: Creates a complete queue of feature extraction pipelines for LOD2 and LOD3 data.

**Parameters**:
- `config`: Application configuration containing SQL file paths
- `lod2Batches`: Array of building ID batches for LOD2 processing
- `lod3Batches`: Array of building ID batches for LOD3 processing

**Returns**: PipelineQueue containing all pipelines ready for execution

**Process**:
1. Creates one pipeline per batch (for both LOD2 and LOD3)
2. Each pipeline contains 8 jobs in sequence:
   - Child Feature Extraction
   - Geometry Dump
   - Feature Extraction
   - Building Feature Extraction
   - Population Density
   - Volume Calculation
   - Storey Calculation
   - Neighbour Calculation

**SQL File Mapping**:
```go
sqlFiles := []struct {
    name string
    path utils.SQLPath
}{
    {"Child Feature Extraction", config.DB.SQLChildFeatureFile},
    {"Geometry Dump", config.DB.SQLGeomDumpFile},
    {"Feature Extraction", config.DB.SQLFeatureAttributesFile},
    {"Building Feature Extraction", config.DB.SQLBuildingFeatureFile},
    {"Population Density", config.DB.SQLPopulationDensityFile},
    {"Volume Calculation", config.DB.SQLVolumeCalcFile},
    {"Storey Calculation", config.DB.SQLStoreyCalcFile},
    {"Neighbour Calculation", config.DB.SQLNeighbourCalcFile},
}
```

**Job Parameters**: All jobs receive these required parameters:
- `lod_schema`: Determines which CityDB schema to read from
- `lod_level`: LOD level (2 or 3)
- `city2tabula_schema`: Target schema for processed data
- `building_ids`: Batch of building IDs to process

**Example Usage**:
```go
lod2Batches := [][]int64{{1001, 1002}, {1003, 1004}}
lod3Batches := [][]int64{{2001, 2002}, {2003, 2004}}

queue := BuildFeatureExtractionQueue(&config, lod2Batches, lod3Batches)
// Result: 4 pipelines, each with 8 jobs, total 32 jobs
```

---

### 7. print.go

Debugging and monitoring utilities for development and troubleshooting.

#### Functions

##### `PrintJobInfo(job *Job)`
**Purpose**: Prints detailed job information for debugging
**Output**: Job ID, type, timestamps, building IDs (first 5), table/schema names

##### `PrintPipelineInfo(pipeline *Pipeline)`
**Purpose**: Prints pipeline summary information
**Output**: Pipeline ID, building ID count, job count

##### `PrintPipelineQueueInfo(queue *PipelineQueue)`
**Purpose**: Prints queue summary statistics
**Output**: Total pipeline count in queue

##### `PrintWorkerInfo(worker *Worker)`
**Purpose**: Prints worker identification information
**Output**: Worker ID

##### `PrintRunnerInfo(runner *Runner)`
**Purpose**: Prints runner context information
**Output**: Associated job ID, pipeline ID, total jobs in pipeline

**Example Output**:
```
Pipeline Queue Details:
----------------------------------
Total Pipelines:         28
----------------------------------
```

---

### 8. database.go

Database setup and schema creation using the job system.

#### Functions

##### `CreateDatabaseJobs(config *utils.Config) []*Job`

**Purpose**: Creates jobs for database schema setup (tabula and training schemas).

**Returns**: Slice of jobs for database initialization

**Jobs Created**:
1. **TABULA_SCHEMA**: Creates tabula tables
2. **TABULA_EXTRACT**: Extracts tabula data 
3. **city2tabula_schema**: Creates training tables

**Parameters**: Uses empty BuildingIDs for schema creation jobs

##### `CreateSchemaIfNotExists(conn *pgxpool.Pool, schemaName string) error`

**Purpose**: Creates PostgreSQL schema if it doesn't exist
**Parameters**: Database connection and schema name
**SQL**: `CREATE SCHEMA IF NOT EXISTS "{schemaName}";`

##### `ExecuteDatabaseSetup(conn *pgxpool.Pool, config *utils.Config) error`

**Purpose**: Executes all database setup jobs in priority order

**Process**:
1. Gets database jobs from CreateDatabaseJobs
2. Executes each job using Runner
3. Returns error if any job fails

##### `CreateDatabaseWithJobs(conn *pgxpool.Pool, config *utils.Config) error`

**Purpose**: Main entry point for unified database creation

**Process**:
1. Creates basic schemas (tabula, training)
2. Creates LOD schemas for configured LOD levels
3. Executes all database setup jobs
4. Replaces all separate db module functions

**Usage**: Called from main.go during `--import_data` phase

---

## Usage Patterns

### 1. Database Setup
```go
// Replace db.SetupDatabaseSchema()
err := process.CreateDatabaseWithJobs(pool, &config)
```

### 2. Feature Extraction
```go
// Create batches
lod2Batches := utils.CreateBatches(lod2BuildingIDs, config.Batch.Size)
lod3Batches := utils.CreateBatches(lod3BuildingIDs, config.Batch.Size)

// Build pipeline queue
queue := process.BuildFeatureExtractionQueue(&config, lod2Batches, lod3Batches)

// Process with workers
pipelineChan := make(chan *process.Pipeline, queue.Len())
var wg sync.WaitGroup

for i := 1; i <= numWorkers; i++ {
    wg.Add(1)
    worker := process.NewWorker(i)
    go worker.Start(pipelineChan, pool, &wg)
}

// Enqueue and wait
for !queue.IsEmpty() {
    pipelineChan <- queue.Dequeue()
}
close(pipelineChan)
wg.Wait()
```

### 3. Individual Job Execution
```go
job := process.NewJob("CUSTOM_JOB", params, sqlFile, 1)
runner := process.NewRunner(job, nil)
err := runner.RunJob(job, pool)
```

## Configuration Integration

The process package integrates with the application configuration:

### SQL File Paths
```go
config.DB.SQLChildFeatureFile      // 01_get_child_feat.sql
config.DB.SQLGeomDumpFile          // 02_dump_child_feat_geom.sql
config.DB.SQLFeatureAttributesFile // 03_calc_child_feat_attr.sql
config.DB.SQLBuildingFeatureFile   // 04_extract_bld_feat.sql
config.DB.SQLPopulationDensityFile // 05_calc_population_density.sql
config.DB.SQLVolumeCalcFile        // 06_calc_volume.sql
config.DB.SQLStoreyCalcFile        // 07_calc_storeys.sql
config.DB.SQLNeighbourCalcFile     // 08_calc_attached_neighbours.sql
config.DB.SQLTabulaTablesFile      // create_tabula_tables.sql
config.DB.SQLTrainingTablesFile    // create_training_tables.sql
```

### Schema Names
```go
config.DB.Schemas.Tabula    // "tabula"
config.DB.Schemas.Training  // "training"
config.DB.Schemas.Lod2      // "lod2"
config.DB.Schemas.Lod3      // "lod3"
```

### Processing Configuration
```go
config.Batch.Size        // Batch size for building IDs
config.Batch.Threads     // Number of concurrent workers
config.CityDB.LODLevels  // Supported LOD levels [2, 3]
```

## Thread Safety

- **PipelineQueue** and **JobQueue**: All operations are mutex-protected
- **Workers**: Safe for concurrent execution via channels
- **Runners**: Stateless execution, safe for concurrent use
- **Database connections**: Uses pgxpool for connection pooling

## Error Handling

- **Job failures**: Return descriptive errors with job context
- **Pipeline failures**: Stop on first job failure, return error
- **Queue operations**: Graceful handling of empty queues
- **SQL execution**: Database errors propagated with context

## Performance Considerations

- **Batching**: Configurable batch sizes prevent memory issues
- **Concurrency**: Worker count matches available CPU cores
- **Connection pooling**: pgxpool manages database connections
- **Parameter reuse**: Efficient string replacement in SQL templates

This documentation provides complete coverage of the process package's functionality, architecture, and usage patterns.
