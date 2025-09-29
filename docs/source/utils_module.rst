Utils Module Documentation
==========================

The ``internal/utils`` package provides essential utility functions for logging, batch processing, database operations, command execution, and data printing throughout the City2TABULA pipeline.

Overview
--------

The utils module contains helper functions used across the entire application:

- **Logging System**: Structured logging with configurable levels
- **Batch Processing**: Efficient data batching for parallel processing
- **Database Utilities**: Common database operations and queries
- **Command Execution**: External command execution with error handling
- **Data Printing**: Formatted output for debugging and monitoring

Package Structure
-----------------

.. code-block:: text

   internal/utils/
   ├── logger.go      # Logging configuration and management
   ├── batch.go       # Batch processing utilities
   ├── citydb.go      # CityDB-specific database operations
   ├── exec.go        # External command execution
   └── print.go       # Data formatting and output utilities

Logging System
--------------

Logger Configuration
~~~~~~~~~~~~~~~~~~~~

The logging system provides structured logging with multiple levels and file output:

**Logger Initialization:**

.. code-block:: go

   // Initialize logger with configuration
   func InitLogger()

   // Logger instances
   var (
       Trace   *log.Logger  // Detailed tracing information
       Debug   *log.Logger  // Debug information
       Info    *log.Logger  // General information
       Warning *log.Logger  // Warning messages
       Error   *log.Logger  // Error messages
   )

**Log Level Configuration:**

.. code-block:: go

   // Supported log levels
   const (
       LogLevelTrace   = "TRACE"
       LogLevelDebug   = "DEBUG"
       LogLevelInfo    = "INFO"
       LogLevelWarning = "WARNING"
       LogLevelError   = "ERROR"
   )

**Log File Management:**

.. code-block:: text

   logs/
   ├── 2025-09-26.log     # Daily log files
   ├── 2025-09-27.log     # Automatic date rotation
   └── 2025-09-28.log     # Structured format

**Log Format:**

.. code-block:: text

   INFO: 2025/09/26 14:22:00 filename.go:123: Message content
   ^     ^                   ^              ^
   Level Timestamp          Source         Message

Usage Examples
~~~~~~~~~~~~~~

.. code-block:: go

   // Import utils package
   import "City2TABULA/internal/utils"

   // Initialize logging (call once at startup)
   utils.InitLogger()

   // Use different log levels
   utils.Info.Println("Processing started")
   utils.Debug.Printf("Processing batch %d of %d", current, total)
   utils.Warning.Println("Large dataset detected, consider increasing memory")
   utils.Error.Printf("Failed to process building %d: %v", buildingID, err)

**Environment Configuration:**

.. code-block:: bash

   # Set log level in .env file
   LOG_LEVEL=INFO

   # Available levels: TRACE, DEBUG, INFO, WARNING, ERROR

Batch Processing Utilities
---------------------------

Batch Creation
~~~~~~~~~~~~~~

Efficient batch processing for handling large datasets:

**Core Functions:**

.. code-block:: go

   // Create batches from slice of building IDs
   func CreateBatches(buildingIDs []int64, batchSize int) [][]int64

   // Calculate optimal batch size based on system resources
   func CalculateOptimalBatchSize(dataSize int64, availableRAM int64,
                                 numWorkers int) int

**Batch Processing Example:**

.. code-block:: go

   // Get building IDs from database
   buildingIDs := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
   batchSize := 3

   // Create batches
   batches := utils.CreateBatches(buildingIDs, batchSize)
   // Result: [[1,2,3], [4,5,6], [7,8,9], [10]]

   // Process each batch
   for i, batch := range batches {
       utils.Info.Printf("Processing batch %d/%d with %d buildings",
                        i+1, len(batches), len(batch))
       processBatch(batch)
   }

**Memory-Aware Batching:**

.. code-block:: go

   // Calculate optimal batch size
   func CalculateOptimalBatchSize(dataSize int64, availableRAM int64,
                                 numWorkers int) int {
       // Estimate memory per building (empirically determined)
       memoryPerBuilding := int64(50 * 1024)  // 50KB per building

       // Calculate maximum buildings that fit in available memory
       maxBuildings := availableRAM / memoryPerBuilding / int64(numWorkers)

       // Ensure reasonable bounds
       batchSize := int(maxBuildings)
       if batchSize < 100 {
           batchSize = 100
       }
       if batchSize > 5000 {
           batchSize = 5000
       }

       return batchSize
   }

Batch Metrics
~~~~~~~~~~~~~

**Performance Tracking:**

.. code-block:: go

   type BatchMetrics struct {
       TotalBatches    int           // Total number of batches
       ProcessedBatches int          // Batches completed
       TotalBuildings  int64         // Total buildings to process
       ProcessedBuildings int64      // Buildings processed
       StartTime       time.Time     // Processing start time
       AverageRate     float64       // Buildings per second
   }

   // Update metrics after batch processing
   func (m *BatchMetrics) UpdateProgress(batchSize int) {
       m.ProcessedBatches++
       m.ProcessedBuildings += int64(batchSize)

       elapsed := time.Since(m.StartTime)
       m.AverageRate = float64(m.ProcessedBuildings) / elapsed.Seconds()

       utils.Info.Printf("Progress: %d/%d batches (%.1f%%), %.0f buildings/sec",
                        m.ProcessedBatches, m.TotalBatches,
                        float64(m.ProcessedBatches)/float64(m.TotalBatches)*100,
                        m.AverageRate)
   }

Database Utilities
------------------

CityDB Operations
~~~~~~~~~~~~~~~~~

Specialized utilities for working with 3D CityDB:

**Building ID Retrieval:**

.. code-block:: go

   // Get all building IDs from specific CityDB schema
   func GetBuildingIDsFromCityDB(pool *pgxpool.Pool, schema string) ([]int64, error)

   // Get building IDs with spatial filtering
   func GetBuildingIDsWithBounds(pool *pgxpool.Pool, schema string,
                                bounds Bounds) ([]int64, error)

**Usage Example:**

.. code-block:: go

   // Get building IDs from LOD2 schema
   lod2BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, "lod2")
   if err != nil {
       utils.Error.Printf("Failed to get LOD2 building IDs: %v", err)
       return err
   }
   utils.Info.Printf("Found %d buildings in LOD2 schema", len(lod2BuildingIDs))

   // Get building IDs from LOD3 schema
   lod3BuildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, "lod3")
   if err != nil {
       utils.Error.Printf("Failed to get LOD3 building IDs: %v", err)
       return err
   }
   utils.Info.Printf("Found %d buildings in LOD3 schema", len(lod3BuildingIDs))

**Query Implementation:**

.. code-block:: go

   func GetBuildingIDsFromCityDB(pool *pgxpool.Pool, schema string) ([]int64, error) {
       query := fmt.Sprintf(`
           SELECT DISTINCT id
           FROM %s.building
           WHERE id IS NOT NULL
           ORDER BY id`, schema)

       rows, err := pool.Query(context.Background(), query)
       if err != nil {
           return nil, fmt.Errorf("query failed: %w", err)
       }
       defer rows.Close()

       var buildingIDs []int64
       for rows.Next() {
           var id int64
           if err := rows.Scan(&id); err != nil {
               return nil, fmt.Errorf("scan failed: %w", err)
           }
           buildingIDs = append(buildingIDs, id)
       }

       return buildingIDs, rows.Err()
   }

Database Health Checks
~~~~~~~~~~~~~~~~~~~~~~

**Connection Validation:**

.. code-block:: go

   // Test database connectivity and performance
   func TestDatabaseConnection(pool *pgxpool.Pool) error {
       ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
       defer cancel()

       // Test basic connectivity
       if err := pool.Ping(ctx); err != nil {
           return fmt.Errorf("database ping failed: %w", err)
       }

       // Test PostGIS extension
       var version string
       err := pool.QueryRow(ctx, "SELECT PostGIS_Version()").Scan(&version)
       if err != nil {
           return fmt.Errorf("PostGIS not available: %w", err)
       }

       utils.Info.Printf("Database connection healthy, PostGIS version: %s", version)
       return nil
   }

**Schema Validation:**

.. code-block:: go

   // Verify required schemas exist
   func ValidateSchemas(pool *pgxpool.Pool, requiredSchemas []string) error {
       query := `
           SELECT schema_name
           FROM information_schema.schemata
           WHERE schema_name = ANY($1)`

       rows, err := pool.Query(context.Background(), query, requiredSchemas)
       if err != nil {
           return err
       }
       defer rows.Close()

       existingSchemas := make(map[string]bool)
       for rows.Next() {
           var schema string
           if err := rows.Scan(&schema); err != nil {
               return err
           }
           existingSchemas[schema] = true
       }

       // Check for missing schemas
       var missing []string
       for _, required := range requiredSchemas {
           if !existingSchemas[required] {
               missing = append(missing, required)
           }
       }

       if len(missing) > 0 {
           return fmt.Errorf("missing schemas: %v", missing)
       }

       return nil
   }

Command Execution Utilities
---------------------------

External Command Execution
~~~~~~~~~~~~~~~~~~~~~~~~~~~

Safe execution of external commands with error handling:

**Core Functions:**

.. code-block:: go

   // Execute command with timeout and error capture
   func ExecuteCommand(cmd *exec.Cmd, timeout time.Duration) ([]byte, error)

   // Execute command with real-time output streaming
   func ExecuteCommandWithOutput(cmd *exec.Cmd,
                                outputHandler func(string)) error

**Usage Examples:**

.. code-block:: go

   // Execute CityDB tool import
   cmd := exec.Command("citydb-tool", "import",
                      "--input", "buildings.gml",
                      "--schema", "lod2")

   output, err := utils.ExecuteCommand(cmd, 10*time.Minute)
   if err != nil {
       utils.Error.Printf("CityDB import failed: %v", err)
       utils.Error.Printf("Command output: %s", string(output))
       return err
   }

   utils.Info.Printf("CityDB import completed successfully")

**Command with Environment:**

.. code-block:: go

   // Execute with custom environment variables
   func ExecuteWithEnvironment(command string, args []string,
                              env map[string]string) error {
       cmd := exec.Command(command, args...)

       // Set environment variables
       for key, value := range env {
           cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
       }

       return cmd.Run()
   }

**Real-time Output Handling:**

.. code-block:: go

   // Stream command output in real-time
   func StreamCommandOutput(cmd *exec.Cmd) error {
       stdout, err := cmd.StdoutPipe()
       if err != nil {
           return err
       }

       stderr, err := cmd.StderrPipe()
       if err != nil {
           return err
       }

       if err := cmd.Start(); err != nil {
           return err
       }

       // Stream stdout
       go func() {
           scanner := bufio.NewScanner(stdout)
           for scanner.Scan() {
               utils.Info.Printf("CMD: %s", scanner.Text())
           }
       }()

       // Stream stderr
       go func() {
           scanner := bufio.NewScanner(stderr)
           for scanner.Scan() {
               utils.Warning.Printf("CMD: %s", scanner.Text())
           }
       }()

       return cmd.Wait()
   }

Data Printing and Formatting
-----------------------------

Pipeline Information Display
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Queue Information:**

.. code-block:: go

   // Print pipeline queue statistics
   func PrintPipelineQueueInfo(queueLength int, jobsPerPipeline int) {
       utils.Info.Printf("Pipeline Queue Information:")
       utils.Info.Printf("  Total Pipelines: %d", queueLength)
       utils.Info.Printf("  Jobs per Pipeline: %d", jobsPerPipeline)
       utils.Info.Printf("  Total Jobs: %d", queueLength*jobsPerPipeline)

       // Estimate processing time
       estimatedMinutes := (queueLength * jobsPerPipeline) / 60  // Rough estimate
       utils.Info.Printf("  Estimated Processing Time: ~%d minutes", estimatedMinutes)
   }

**Performance Metrics Display:**

.. code-block:: go

   // Display processing performance metrics
   func PrintPerformanceMetrics(startTime time.Time, buildingsProcessed int64,
                               jobsCompleted int64) {
       elapsed := time.Since(startTime)
       buildingRate := float64(buildingsProcessed) / elapsed.Seconds()
       jobRate := float64(jobsCompleted) / elapsed.Seconds()

       utils.Info.Printf("Performance Metrics:")
       utils.Info.Printf("  Total Runtime: %v", elapsed)
       utils.Info.Printf("  Buildings Processed: %d", buildingsProcessed)
       utils.Info.Printf("  Jobs Completed: %d", jobsCompleted)
       utils.Info.Printf("  Building Processing Rate: %.2f buildings/sec", buildingRate)
       utils.Info.Printf("  Job Processing Rate: %.2f jobs/sec", jobRate)
   }

**Data Summary Tables:**

.. code-block:: go

   // Print tabular data summary
   func PrintDataSummary(data map[string]interface{}) {
       utils.Info.Println("Data Summary:")
       utils.Info.Println("+" + strings.Repeat("-", 50) + "+")

       for key, value := range data {
           utils.Info.Printf("| %-20s | %-25v |", key, value)
       }

       utils.Info.Println("+" + strings.Repeat("-", 50) + "+")
   }

Memory and System Monitoring
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Memory Usage Tracking:**

.. code-block:: go

   // Monitor and log memory usage
   func LogMemoryUsage(context string) {
       var m runtime.MemStats
       runtime.ReadMemStats(&m)

       utils.Debug.Printf("Memory Usage [%s]:", context)
       utils.Debug.Printf("  Allocated: %d KB", bToKb(m.Alloc))
       utils.Debug.Printf("  Total Allocated: %d KB", bToKb(m.TotalAlloc))
       utils.Debug.Printf("  System Memory: %d KB", bToKb(m.Sys))
       utils.Debug.Printf("  GC Cycles: %d", m.NumGC)
   }

   func bToKb(b uint64) uint64 {
       return b / 1024
   }

**System Resource Monitoring:**

.. code-block:: go

   // Monitor system resources
   func MonitorSystemResources() {
       // Get CPU usage
       cpuPercent, _ := cpu.Percent(time.Second, false)

       // Get memory statistics
       memInfo, _ := mem.VirtualMemory()

       // Get disk usage
       diskInfo, _ := disk.Usage("/")

       utils.Debug.Printf("System Resources:")
       utils.Debug.Printf("  CPU Usage: %.2f%%", cpuPercent[0])
       utils.Debug.Printf("  Memory Usage: %.2f%% (%d MB used / %d MB total)",
                         memInfo.UsedPercent,
                         memInfo.Used/1024/1024,
                         memInfo.Total/1024/1024)
       utils.Debug.Printf("  Disk Usage: %.2f%% (%d GB used / %d GB total)",
                         diskInfo.UsedPercent,
                         diskInfo.Used/1024/1024/1024,
                         diskInfo.Total/1024/1024/1024)
   }

Error Handling and Recovery
---------------------------

Error Classification
~~~~~~~~~~~~~~~~~~~~

**Error Types:**

.. code-block:: go

   // Common error types with specific handling
   var (
       ErrDatabaseConnection = errors.New("database connection failed")
       ErrInvalidData       = errors.New("invalid data format")
       ErrResourceExhausted = errors.New("system resources exhausted")
       ErrTimeoutExceeded   = errors.New("operation timeout exceeded")
   )

**Error Context Enhancement:**

.. code-block:: go

   // Add context to errors for better debugging
   func WrapError(err error, context string, details map[string]interface{}) error {
       if err == nil {
           return nil
       }

       var detailsStr strings.Builder
       for key, value := range details {
           detailsStr.WriteString(fmt.Sprintf("%s=%v ", key, value))
       }

       return fmt.Errorf("%s: %w (details: %s)", context, err, detailsStr.String())
   }

**Retry Utilities:**

.. code-block:: go

   // Generic retry function with exponential backoff
   func RetryWithBackoff(operation func() error, maxAttempts int,
                        baseDelay time.Duration) error {
       var lastErr error

       for attempt := 1; attempt <= maxAttempts; attempt++ {
           if err := operation(); err != nil {
               lastErr = err

               if attempt < maxAttempts {
                   delay := baseDelay * time.Duration(1<<uint(attempt-1))
                   utils.Warning.Printf("Attempt %d failed, retrying in %v: %v",
                                       attempt, delay, err)
                   time.Sleep(delay)
               }
           } else {
               return nil  // Success
           }
       }

       return fmt.Errorf("operation failed after %d attempts: %w",
                        maxAttempts, lastErr)
   }

Usage Examples
--------------

Complete Utility Usage
~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   package main

   import (
       "City2TABULA/internal/config"
       "City2TABULA/internal/db"
       "City2TABULA/internal/utils"
   )

   func main() {
       // Initialize logging
       utils.InitLogger()
       utils.Info.Println("City2TABULA starting...")

       // Load configuration
       config := config.LoadConfig()

       // Connect to database
       pool, err := db.ConnectPool(config)
       if err != nil {
           utils.Error.Fatalf("Database connection failed: %v", err)
       }
       defer pool.Close()

       // Test database health
       if err := utils.TestDatabaseConnection(pool); err != nil {
           utils.Error.Fatalf("Database health check failed: %v", err)
       }

       // Get building IDs and create batches
       buildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, "lod2")
       if err != nil {
           utils.Error.Fatalf("Failed to get building IDs: %v", err)
       }

       batches := utils.CreateBatches(buildingIDs, config.Batch.Size)
       utils.PrintPipelineQueueInfo(len(batches), 8)  // 8 jobs per pipeline

       // Process batches with performance monitoring
       startTime := time.Now()
       for i, batch := range batches {
           utils.Info.Printf("Processing batch %d/%d", i+1, len(batches))

           // Process batch (placeholder)
           processBatch(batch)

           // Log memory usage periodically
           if i%10 == 0 {
               utils.LogMemoryUsage(fmt.Sprintf("Batch %d", i))
           }
       }

       // Final performance metrics
       utils.PrintPerformanceMetrics(startTime, int64(len(buildingIDs)),
                                   int64(len(batches)*8))
   }

Error Handling Example
~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Robust database operation with retry
   func ProcessWithRetry(pool *pgxpool.Pool, buildingID int64) error {
       operation := func() error {
           return processBuilding(pool, buildingID)
       }

       return utils.RetryWithBackoff(operation, 3, 100*time.Millisecond)
   }

   // Enhanced error reporting
   func ProcessBuildingWithContext(pool *pgxpool.Pool, buildingID int64) error {
       err := processBuilding(pool, buildingID)
       if err != nil {
           return utils.WrapError(err, "building processing failed",
                                 map[string]interface{}{
                                     "building_id": buildingID,
                                     "timestamp": time.Now(),
                                 })
       }
       return nil
   }

For more information on configuration and database operations, see :doc:`config_module` and :doc:`database_module`.