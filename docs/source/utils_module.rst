Utils Module Documentation
==========================

The ``internal/utils`` package provides essential utility functions for logging, batch processing, database operations, command execution, and data printing throughout the City2TABULA pipeline.

Overview
--------

The utils module contains simple, focused helper functions used across the application:

- **Logging System**: File and console logging with configurable levels
- **Batch Processing**: Simple data batching for parallel processing
- **Database Utilities**: Building ID retrieval from CityDB schemas
- **Command Execution**: Shell command execution for CityDB operations
- **Data Printing**: Formatted output for pipeline information

Package Structure
-----------------

.. code-block:: text

   internal/utils/
   ├── logger.go      # Logging setup and configuration
   ├── batch.go       # Simple batch creation utility
   ├── citydb.go      # CityDB building ID queries
   ├── exec.go        # Command execution for CityDB scripts
   └── print.go       # Pipeline and job information display

Logging System (logger.go)
---------------------------

Simple logging system with file output and configurable levels.

**Logger Initialization:**

.. code-block:: go

   // Initialize logger - creates daily log files in logs/ directory
   func InitLogger()

   // Available loggers
   var (
       Info  *log.Logger  // General information
       Debug *log.Logger  // Debug information (controlled by LOG_LEVEL)
       Warn  *log.Logger  // Warning messages
       Error *log.Logger  // Error messages
   )

**Configuration:**

.. code-block:: bash

   # Set log level in .env file
   LOG_LEVEL=DEBUG    # Shows debug messages
   LOG_LEVEL=INFO     # Default - shows info, warn, error
   LOG_LEVEL=WARN     # Shows only warnings and errors
   LOG_LEVEL=ERROR    # Shows only errors

**Usage:**

.. code-block:: go

   // Initialize once at startup
   utils.InitLogger()

   // Use throughout application
   utils.Info.Println("Processing started")
   utils.Debug.Printf("Processing building %d", buildingID)
   utils.Warn.Println("Large dataset detected")
   utils.Error.Printf("Failed to process: %v", err)

**Log Files:**

.. code-block:: text

   logs/
   ├── 2025-10-08.log     # Daily log files
   └── 2025-10-09.log     # Automatic date-based rotation

Batch Processing (batch.go)
----------------------------

Simple utility for creating batches from building ID slices.

**Core Function:**

.. code-block:: go

   // Create batches from slice of building IDs
   func CreateBatches(ids []int64, batchSize int) [][]int64

**Usage:**

.. code-block:: go

   buildingIDs := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
   batchSize := 3

   batches := utils.CreateBatches(buildingIDs, batchSize)
   // Result: [[1,2,3], [4,5,6], [7,8,9], [10]]

   // Process each batch
   for i, batch := range batches {
       utils.Info.Printf("Processing batch %d/%d with %d buildings",
                        i+1, len(batches), len(batch))
       // Process batch...
   }

Database Utilities (citydb.go)
-------------------------------

Simple functions for retrieving building information from CityDB schemas.

**Available Functions:**

.. code-block:: go

   // Get count of buildings in schema
   func GetBuildingFeatureCount(dbConn *pgxpool.Pool, schemaName string) (int, error)

   // Get all building IDs from schema
   func GetBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error)

**Usage:**

.. code-block:: go

   // Get building count
   count, err := utils.GetBuildingFeatureCount(pool, "lod2")
   if err != nil {
       utils.Error.Printf("Failed to get building count: %v", err)
       return err
   }
   utils.Info.Printf("Found %d buildings in LOD2 schema", count)

   // Get building IDs
   buildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, "lod2")
   if err != nil {
       utils.Error.Printf("Failed to get building IDs: %v", err)
       return err
   }

**Implementation Details:**

The functions query CityDB's ``feature`` table looking for building objects (``objectclass_id`` 901 and 905).

Command Execution (exec.go)
----------------------------

Utilities for executing shell commands, particularly CityDB scripts.

**Core Functions:**

.. code-block:: go

   // Execute CityDB SQL script with parameters
   func ExecuteCityDBScript(config *config.Config, sqlFilePath string,
                           schemaName string) error

   // Execute general shell command
   func ExecuteCommand(command string) error

**Usage:**

.. code-block:: go

   // Execute CityDB script
   err := utils.ExecuteCityDBScript(config, "create_schema.sql", "lod2")
   if err != nil {
       utils.Error.Printf("CityDB script failed: %v", err)
       return err
   }

   // Execute general command
   err = utils.ExecuteCommand("citydb-tool import --input data.gml")
   if err != nil {
       utils.Error.Printf("Command failed: %v", err)
       return err
   }

The CityDB script execution automatically handles database connection parameters and variable substitution for SRID and schema names.

Data Printing (print.go)
-------------------------

Formatted output utilities for displaying pipeline and job information.

**Available Functions:**

.. code-block:: go

   // Print detailed job information
   func PrintJobInfo(jobID, jobType string, createdAt time.Time,
                    buildingIDs []int64, tableNames, schemaNames []string)

   // Print pipeline information
   func PrintPipelineInfo(pipelineID string, buildingIDs []int64, jobCount int)

   // Print pipeline queue statistics
   func PrintPipelineQueueInfo(totalPipelines int, totalJobsInPipeline int)

**Usage:**

.. code-block:: go

   // Print job details
   utils.PrintJobInfo(
       "job-123",
       "feature_extraction",
       time.Now(),
       buildingIDs,
       []string{"building_feature"},
       []string{"lod2"}
   )

   // Print pipeline information
   utils.PrintPipelineInfo("pipeline-456", buildingIDs, 8)

   // Print queue statistics
   utils.PrintPipelineQueueInfo(10, 8)  // 10 pipelines, 8 jobs each

**Sample Output:**

.. code-block:: text

   Job Details:
   ----------------------------------
   Job ID               : job-123
   Job Type             : feature_extraction
   Created At           : 2025-10-08 14:22:00
   Total Building IDs   : 1000
   Building IDs         : [1, 2, 3, 4, 5]...
   Table Names          : [building_feature]
   Schema Names         : [lod2]
   ----------------------------------

   Pipeline Queue Details:
   ----------------------------------
   Total Pipelines        : 10
   Total Jobs per Pipeline: 8
   Total Jobs             : 80
   ----------------------------------

Complete Usage Example
----------------------

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

       // Load configuration and connect to database
       config := config.LoadConfig()
       pool, err := db.ConnectPool(config)
       if err != nil {
           utils.Error.Fatalf("Database connection failed: %v", err)
       }
       defer pool.Close()

       // Get building IDs and create batches
       buildingIDs, err := utils.GetBuildingIDsFromCityDB(pool, "lod2")
       if err != nil {
           utils.Error.Fatalf("Failed to get building IDs: %v", err)
       }

       batches := utils.CreateBatches(buildingIDs, 1000)
       utils.PrintPipelineQueueInfo(len(batches), 8)

       // Process batches
       for i, batch := range batches {
           utils.Info.Printf("Processing batch %d/%d", i+1, len(batches))
           // Process batch...
       }

       utils.Info.Println("Processing complete")
   }

For more information on configuration and database operations, see :doc:`config_module` and :doc:`database_module`.