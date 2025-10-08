Process Module Documentation
=============================

The ``internal/process`` package provides the core processing engine for City2TABULA, implementing parallel batch processing with job queues, worker pools, and pipeline orchestration.

Overview
--------

The process module implements a sophisticated parallel processing architecture:

- **Job-Based Processing**: Individual SQL operations as discrete jobs
- **Pipeline Organization**: Jobs grouped into logical processing pipelines
- **Worker Pool Pattern**: Configurable number of concurrent workers
- **Queue Management**: Thread-safe pipeline and job queuing
- **Retry Logic**: Automatic retry with exponential backoff
- **Performance Monitoring**: Real-time processing metrics

Package Structure
-----------------

.. code-block:: text

   internal/process/
   ├── job.go           # Job definition and management
   ├── pipeline.go      # Pipeline structure and operations
   ├── queue.go         # Thread-safe pipeline queuing
   ├── orchestrator.go  # Pipeline creation and orchestration
   ├── worker.go        # Worker pool implementation
   └── runner.go        # Job execution and retry logic

Architecture Overview
---------------------

Processing Flow
~~~~~~~~~~~~~~~

.. code-block:: text

   Data Input → Pipeline Queue → Worker Pool → Job Execution → Results
        ↓              ↓              ↓             ↓            ↓
   [Buildings]   [SQL Scripts]   [Goroutines]   [Database]   [Features]

**Key Components:**

1. **Orchestrator**: Creates pipelines based on configuration and data
2. **Queue**: Thread-safe storage for pipelines awaiting processing
3. **Workers**: Concurrent goroutines that process pipelines
4. **Runner**: Executes individual jobs with retry logic
5. **Jobs**: Atomic units of work (typically SQL script execution)

Core Components
---------------

Job Management
~~~~~~~~~~~~~~

**Job Structure:**

.. code-block:: go

   type Job struct {
       ID          string                 // Unique job identifier
       Name        string                 // Human-readable job name
       Params      *Params               // Job parameters (building IDs, etc.)
       SQLFile     string                // Path to SQL script
       Priority    int                   // Execution priority
       CreatedAt   time.Time             // Job creation timestamp
       StartedAt   *time.Time            // Job start time
       CompletedAt *time.Time            // Job completion time
       Error       error                 // Error information if failed
   }

**Job Parameters:**

.. code-block:: go

   type Params struct {
       BuildingIDs []int64               // Building IDs for batch processing
       // Additional parameters can be added as needed
   }

**Job Creation:**

.. code-block:: go

   // Create a new job
   job := NewJob("Surface Analysis", params, "sql/scripts/main/03_calc_surface.sql", 1)

Pipeline Management
~~~~~~~~~~~~~~~~~~~

**Pipeline Structure:**

.. code-block:: go

   type Pipeline struct {
       ID          string                // Unique pipeline identifier
       BuildingIDs []int64              // Buildings processed by this pipeline
       Jobs        []*Job               // Jobs in execution order
       Status      PipelineStatus       // Current pipeline status
       CreatedAt   time.Time            // Pipeline creation time
       StartedAt   *time.Time           // Pipeline start time
       CompletedAt *time.Time           // Pipeline completion time
       EnqueuedAt  time.Time            // Queue entry time
   }

**Pipeline Operations:**

.. code-block:: go

   // Create new pipeline
   pipeline := NewPipeline(buildingIDs, nil)

   // Add jobs to pipeline
   pipeline.AddJob(job1)
   pipeline.AddJob(job2)

   // Execute pipeline
   worker.ProcessPipeline(pipeline, dbPool, config)

Queue Management
~~~~~~~~~~~~~~~~

**Thread-Safe Pipeline Queue:**

.. code-block:: go

   type PipelineQueue struct {
       pipelines []*Pipeline
       mutex     sync.RWMutex          // Thread-safe access
   }

**Queue Operations:**

.. code-block:: go

   // Create queue
   queue := NewPipelineQueue()

   // Add pipeline to queue
   queue.Enqueue(pipeline)

   // Remove pipeline from queue
   pipeline := queue.Dequeue()

   // Check queue status
   length := queue.Len()
   isEmpty := queue.IsEmpty()

Worker Pool Implementation
--------------------------

Worker Architecture
~~~~~~~~~~~~~~~~~~~

**Worker Structure:**

.. code-block:: go

   type Worker struct {
       ID       int                     // Worker identifier
       // Additional worker state
   }

**Worker Lifecycle:**

.. code-block:: go

   // Create worker
   worker := NewWorker(workerID)

   // Start worker (runs in goroutine)
   go worker.Start(pipelineChannel, dbPool, &waitGroup, config)

**Parallel Processing Pattern:**

.. code-block:: go

   // Set up worker pool
   numWorkers := config.Batch.Threads
   var wg sync.WaitGroup

   // Create pipeline channel
   pipelineChan := make(chan *Pipeline, queue.Len())

   // Start workers
   for i := 1; i <= numWorkers; i++ {
       wg.Add(1)
       worker := NewWorker(i)
       go worker.Start(pipelineChan, pool, &wg, config)
   }

   // Enqueue pipelines
   for !queue.IsEmpty() {
       pipeline := queue.Dequeue()
       if pipeline != nil {
           pipelineChan <- pipeline
       }
   }
   close(pipelineChan)

   // Wait for completion
   wg.Wait()

Job Execution and Retry Logic
-----------------------------

Job Runner
~~~~~~~~~~

**Execution Flow:**

.. code-block:: go

   type Runner struct {
       config *config.Config
   }

   // Execute job with retry logic
   func (r *Runner) RunJobWithRetry(job *Job, pool *pgxpool.Pool,
                                   config *config.Config, attempt int) error

**Retry Configuration:**

.. code-block:: go

   type RetryConfig struct {
       MaxAttempts    int           // Maximum retry attempts (default: 3)
       BaseDelay      time.Duration // Base delay between retries (default: 100ms)
       MaxDelay       time.Duration // Maximum delay (default: 5s)
       BackoffFactor  float64       // Exponential backoff multiplier (default: 2.0)
   }

**Retry Behavior:**

1. **Initial Attempt**: Execute job immediately
2. **First Retry**: Wait 100ms, then retry
3. **Second Retry**: Wait 200ms, then retry
4. **Third Retry**: Wait 400ms, then retry
5. **Failure**: Log error and mark job as failed

SQL Execution
~~~~~~~~~~~~~

**Template Processing:**

.. code-block:: go

   // SQL templates support dynamic parameter substitution
   func (r *Runner) ProcessSQLTemplate(sqlContent string, params *Params,
                                      config *config.Config) string

**Supported Placeholders:**

- ``{building_ids}``: Replaced with SQL array of building IDs
- ``{city2tabula_schema}``: Replaced with training schema name
- ``{lod_schema}``: Replaced with appropriate LOD schema (lod2/lod3)
- ``{tabula_schema}``: Replaced with tabula schema name

**Example SQL Template:**

.. code-block:: sql

   -- Template: sql/scripts/main/example.sql
   SELECT building_id, surface_area
   FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
   WHERE building_id IN {building_ids}
   AND surface_area > 0;

Pipeline Orchestration
----------------------

Orchestrator Functions
~~~~~~~~~~~~~~~~~~~~~~

**Feature Extraction Pipeline:**

.. code-block:: go

   // Create pipeline queue for feature extraction
   func BuildFeatureExtractionQueue(config *Config,
                                   lod2Batches [][]int64,
                                   lod3Batches [][]int64) (*PipelineQueue, error)

**Database Setup Pipeline:**

.. code-block:: go

   // Create pipeline for database schema setup
   func DBSetupPipelineQueue(config *Config) (*PipelineQueue, error)

**Supplementary Data Pipeline:**

.. code-block:: go

   // Create pipeline for supplementary data import
   func SupplementaryPipelineQueue(config *Config) (*PipelineQueue, error)

Pipeline Types
~~~~~~~~~~~~~~

**1. Database Setup Pipeline:**

Sequential execution of schema creation scripts:

.. code-block:: text

   Schema Tables → Supplementary Scripts → Function Scripts

**2. Feature Extraction Pipeline:**

Parallel processing of building batches:

.. code-block:: text

   LOD2 Batch 1: [Jobs 1-8] → Results
   LOD2 Batch 2: [Jobs 1-8] → Results
   LOD3 Batch 1: [Jobs 1-8] → Results
   LOD3 Batch 2: [Jobs 1-8] → Results

**3. Supplementary Pipeline:**

Import and processing of reference data:

.. code-block:: text

   TABULA Data → Attribute Extraction → Validation

Performance Optimization
------------------------

Batch Processing
~~~~~~~~~~~~~~~~

**Recovery Strategies:**

1. **Job Retry**: Automatic retry with exponential backoff
2. **Pipeline Restart**: Resume from failed job
3. **Graceful Degradation**: Continue processing other pipelines
4. **Resource Recovery**: Reconnect to database, reallocate memory


Logging and Debugging
~~~~~~~~~~~~~~~~~~~~~

**Structured Logging:**

.. code-block:: go

   // Worker logging
   utils.Info.Printf("[Worker %d] Starting pipeline %s",
                    worker.ID, pipeline.ID)

   // Job logging
   utils.Info.Printf("[Worker %d] Starting job: %s (SQL file: %s)",
                    worker.ID, job.Name, job.SQLFile)

   // Performance logging
   utils.Info.Printf("[Worker %d] Pipeline %s completed in %v",
                    worker.ID, pipeline.ID, duration)

Usage Examples
--------------

Basic Pipeline Processing
~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   package main

   import (
       "City2TABULA/internal/config"
       "City2TABULA/internal/process"
       "City2TABULA/internal/utils"
   )

   func main() {
       config := config.LoadConfig()

       // Get building IDs from database
       lod2IDs, _ := utils.GetBuildingIDsFromCityDB(pool, "lod2")
       lod3IDs, _ := utils.GetBuildingIDsFromCityDB(pool, "lod3")

       // Create batches
       lod2Batches := utils.CreateBatches(lod2IDs, config.Batch.Size)
       lod3Batches := utils.CreateBatches(lod3IDs, config.Batch.Size)

       // Build processing queue
       queue, err := process.BuildFeatureExtractionQueue(config,
                                                        lod2Batches,
                                                        lod3Batches)
       if err != nil {
           log.Fatalf("Failed to build queue: %v", err)
       }

       // Set up worker pool
       numWorkers := config.Batch.Threads
       var wg sync.WaitGroup
       pipelineChan := make(chan *process.Pipeline, queue.Len())

       // Start workers
       for i := 1; i <= numWorkers; i++ {
           wg.Add(1)
           worker := process.NewWorker(i)
           go worker.Start(pipelineChan, pool, &wg, config)
       }

       // Enqueue pipelines
       for !queue.IsEmpty() {
           pipeline := queue.Dequeue()
           if pipeline != nil {
               pipelineChan <- pipeline
           }
       }
       close(pipelineChan)

       // Wait for completion
       wg.Wait()
   }

Custom Pipeline Creation
~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Create custom pipeline for specific processing
   func CreateCustomPipeline(buildingIDs []int64, config *config.Config) *Pipeline {
       pipeline := process.NewPipeline(buildingIDs, nil)

       params := &process.Params{
           BuildingIDs: buildingIDs,
       }

       // Add custom jobs
       pipeline.AddJob(process.NewJob("Custom Analysis 1", params,
                                     "sql/custom/analysis1.sql", 1))
       pipeline.AddJob(process.NewJob("Custom Analysis 2", params,
                                     "sql/custom/analysis2.sql", 2))

       return pipeline
   }

For more information on configuration, see :doc:`config_module` and :doc:`database_module`.