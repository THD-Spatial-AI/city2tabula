Process Module
==============

The ``internal/process`` package implements parallel batch processing for City2TABULA's feature extraction pipeline.

Core Architecture
-----------------

**Components:**
- **Jobs**: Atomic SQL operations with parameters and priority
- **Pipelines**: Sequential job collections for batch processing
- **Queue**: Thread-safe pipeline storage and management
- **Workers**: Concurrent goroutines executing pipelines
- **Runner**: Job execution engine with retry logic
- **Orchestrator**: Pipeline creation and coordination

**Processing Flow:**

.. code-block:: text

   Building Batches → Pipelines → Queue → Workers → SQL Execution

Job System (job.go)
-------------------

Jobs represent individual SQL operations with building ID batches:

.. code-block:: go

   type Job struct {
       JobID     uuid.UUID  // Unique identifier
       JobType   string     // e.g., "LOD2 01_get_child_feat.sql"
       Params    *Params    // Building IDs for processing
       SQLFile   string     // SQL script path
       Priority  int        // Execution order (lower = higher priority)
       CreatedAt time.Time  // Creation timestamp
   }

   type Params struct {
       BuildingIDs []int64  // Buildings to process in this job
   }

**Usage:**

.. code-block:: go

   params := &Params{BuildingIDs: []int64{1, 2, 3}}
   job := NewJob("Surface Analysis", params, "sql/scripts/main/03_calc.sql", 1)

Pipeline Management (pipeline.go)
---------------------------------

Pipelines group jobs for sequential execution:

.. code-block:: go

   type Pipeline struct {
       PipelineID  uuid.UUID  // Unique identifier
       BuildingIDs []int64    // All buildings in this pipeline
       Jobs        []*Job     // Jobs to execute in order
       EnqueuedAt  time.Time  // Queue timestamp
       CreatedAt   string     // Creation timestamp
   }

**Usage:**

.. code-block:: go

   pipeline := NewPipeline(buildingIDs, nil)
   pipeline.AddJob(job1)
   pipeline.AddJob(job2)

Queue System (queue.go)
-----------------------

Thread-safe pipeline queue with FIFO processing:

.. code-block:: go

   type PipelineQueue struct {
       pipelines []*Pipeline  // Pipeline storage
       mutex     sync.RWMutex // Thread safety
   }

**Operations:**

.. code-block:: go

   queue := NewPipelineQueue()
   queue.Enqueue(pipeline)           // Add pipeline
   pipeline := queue.Dequeue()       // Get next pipeline
   isEmpty := queue.IsEmpty()        // Check if empty
   length := queue.Len()             // Get queue size

Worker Pool (worker.go)
-----------------------

Concurrent workers process pipelines from the queue:

.. code-block:: go

   type Worker struct {
       ID int  // Worker identifier
   }

**Worker Execution:**

.. code-block:: go

   worker := NewWorker(1)
   worker.Start(pipelineChan, dbConn, &waitGroup, config)

Workers automatically:
- Retrieve pipelines from the channel
- Execute all jobs in priority order
- Handle errors and logging
- Signal completion via WaitGroup

Job Execution (runner.go)
-------------------------

The Runner executes jobs with retry logic and error handling:

.. code-block:: go

   type Runner struct {
       config *config.Config  // Configuration reference
   }

**Key Features:**

- **Priority Execution**: Jobs sorted by priority before execution
- **Retry Logic**: Exponential backoff for failed jobs
- **Deadlock Handling**: Special retry logic for database deadlocks
- **LOD Detection**: Automatic LOD level extraction from job types

**Retry Configuration:**

.. code-block:: go

   // Regular retries: exponential backoff
   maxRetries := config.RetryConfig.MaxRetries
   delay := initialDelay * math.Pow(backoffFactor, attempt)

   // Deadlock retries: randomized short delays
   deadlockDelay := (50 + attempt*25)ms + randomJitter

**Usage:**

.. code-block:: go

   runner := NewRunner(config)
   err := runner.RunPipeline(pipeline, dbConn, workerID)

Pipeline Orchestration (orchestrator.go)
----------------------------------------

Creates specialized pipeline queues for different processing stages:

**Feature Extraction Pipelines:**

.. code-block:: go

   queue, err := BuildFeatureExtractionQueue(config, lod2Batches, lod3Batches)

- Creates one pipeline per building batch
- Separate pipelines for LOD2 and LOD3 data
- Jobs ordered by SQL script sequence (01, 02, 03...)

**Database Setup Pipelines:**

.. code-block:: go

   queue, err := DBSetupPipelineQueue(config)

- Single pipeline for schema/table creation
- Includes table scripts and function scripts
- Executed before feature extraction

**Supplementary Pipelines:**

.. code-block:: go

   queue, err := SupplementaryPipelineQueue(config)

- Single pipeline for supplementary scripts
- Supporting operations and utilities
- Executed after main processing

Usage Examples
--------------

**Complete Processing Workflow:**

.. code-block:: go

   import "City2TABULA/internal/process"

   // 1. Create building batches
   lod2Batches := [][]int64{{1,2,3}, {4,5,6}}
   lod3Batches := [][]int64{{1,2,3}, {4,5,6}}

   // 2. Build pipeline queue
   queue, err := process.BuildFeatureExtractionQueue(config, lod2Batches, lod3Batches)
   if err != nil {
       log.Fatal(err)
   }

   // 3. Create worker pool
   workerCount := config.Batch.ThreadCount
   pipelineChan := make(chan *process.Pipeline, queue.Len())
   var wg sync.WaitGroup

   // 4. Start workers
   for i := 0; i < workerCount; i++ {
       wg.Add(1)
       worker := process.NewWorker(i)
       go worker.Start(pipelineChan, dbConn, &wg, config)
   }

   // 5. Feed pipelines to workers
   for !queue.IsEmpty() {
       pipeline := queue.Dequeue()
       pipelineChan <- pipeline
   }
   close(pipelineChan)

   // 6. Wait for completion
   wg.Wait()

**Individual Job Execution:**

.. code-block:: go

   // Create job
   params := &process.Params{BuildingIDs: []int64{1, 2, 3}}
   job := process.NewJob("LOD2 Surface Analysis", params, "sql/scripts/main/03_calc.sql", 1)

   // Execute with retry
   runner := process.NewRunner(config)
   err := runner.RunJobWithRetry(job, dbConn, config, workerID)

Performance Considerations
-------------------------

**Optimal Batch Sizing:**
- Balance memory usage vs. parallelism
- Typical batch sizes: 1000-5000 buildings
- Monitor database connection limits

**Worker Configuration:**
- Default: Number of CPU cores
- Consider database connection pool size
- Monitor deadlock frequency

**Memory Management:**
- Workers process pipelines sequentially
- No pipeline caching in memory
- Garbage collection after pipeline completion

