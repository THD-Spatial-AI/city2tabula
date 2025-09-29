Importer Module Documentation
=============================

The ``internal/importer`` package handles the import of various data sources into the City2TABULA database, including 3D CityDB data, TABULA building type classifications, and supplementary reference data.

Overview
--------

The importer module provides specialized importers for different data formats:

- **CityDB Data Import**: 3D building geometries from CityGML files
- **TABULA Data Import**: Building type classifications from CSV files
- **Supplementary Data Import**: Additional reference data and SQL processing
- **Batch Processing**: Efficient import of large datasets
- **Error Handling**: Robust error handling and recovery mechanisms

Package Structure
-----------------

.. code-block:: text

   internal/importer/
   ├── citydb.go         # CityDB/CityGML data import
   ├── supplementary.go  # Supplementary data and SQL processing
   └── (additional importers as needed)

CityDB Data Import
------------------

CityGML Import Process
~~~~~~~~~~~~~~~~~~~~~~

The CityDB importer handles 3D building data in CityGML format using the external CityDB Tool:

**Core Functions:**

.. code-block:: go

   // Import all CityDB data (LOD2 and LOD3)
   func ImportCityDBData(pool *pgxpool.Pool, config *config.Config) error

   // Import specific LOD level data
   func ImportLODData(pool *pgxpool.Pool, config *config.Config,
                     lodLevel string) error

**Import Process:**

1. **Discovery**: Scan data directories for CityGML files
2. **Validation**: Verify file formats and accessibility
3. **Import**: Use CityDB Tool to import data into PostgreSQL
4. **Indexing**: Create spatial indexes for optimal query performance
5. **Validation**: Verify import success and data integrity

Data Directory Structure
~~~~~~~~~~~~~~~~~~~~~~~~

**Expected Directory Layout:**

.. code-block:: text

   data/
   ├── lod2/
   │   ├── austria/
   │   │   └── vienna/
   │   │       └── buildings.gml
   │   ├── germany/
   │   │   ├── deggendorf/
   │   │   │   └── buildings.gml
   │   │   └── munich/
   │   │       └── buildings.gml
   │   └── netherlands/
   │       └── amsterdam/
   │           └── buildings.gml
   └── lod3/
       ├── austria/
       ├── germany/
       └── netherlands/

**File Format Support:**

- **CityGML 2.0**: Full support for building features
- **CityGML 3.0**: Compatible with newer format versions
- **GML**: Standard Geography Markup Language files
- **Compressed Files**: Support for .gz compressed files

CityDB Tool Integration
~~~~~~~~~~~~~~~~~~~~~~~

**Tool Configuration:**

.. code-block:: go

   type CityDBConfig struct {
       ToolPath    string  // Path to citydb-tool executable
       JavaOpts    string  // JVM options for performance tuning
       TempDir     string  // Temporary directory for processing
       LogLevel    string  // Logging verbosity
   }

**Import Command Construction:**

.. code-block:: go

   // Build citydb-tool import command
   func buildImportCommand(config *config.Config, gmlFile string,
                          schema string) *exec.Cmd {

       args := []string{
           "import",
           "--db-host", config.DB.Host,
           "--db-port", config.DB.Port,
           "--db-name", config.DB.Name,
           "--db-user", config.DB.User,
           "--db-schema", schema,
           "--input-file", gmlFile,
       }

       cmd := exec.Command(config.CityDB.ToolPath, args...)
       cmd.Env = append(os.Environ(),
                       fmt.Sprintf("PGPASSWORD=%s", config.DB.Password))

       return cmd
   }

**Performance Optimization:**

.. code-block:: bash

   # Java heap size optimization for large files
   export JAVA_OPTS="-Xmx8g -Xms2g"

   # Parallel import settings
   --threads 4
   --batch-size 1000

Error Handling
~~~~~~~~~~~~~~

**Common Import Issues:**

1. **Invalid CityGML Files**:

   .. code-block:: text

      Error: XML parsing failed
      Solution: Validate CityGML files against schema

2. **Memory Issues**:

   .. code-block:: text

      Error: Java heap space
      Solution: Increase Java heap size with JAVA_OPTS

3. **Schema Conflicts**:

   .. code-block:: text

      Error: Table already exists
      Solution: Reset CityDB schema before import

4. **Permission Issues**:

   .. code-block:: text

      Error: Access denied for user
      Solution: Grant appropriate database privileges

TABULA Data Import
------------------

CSV Data Import
~~~~~~~~~~~~~~~

note:: TABULA data import from CSV will be replaced with a go based importer in future releases.
**TABULA CSV Structure:**

.. code-block:: csv

   id,country,year_of_construction,building_type,heating_type,tabula_class,...
   1,DE,1975,SFH,gas,DE.N.SFH.07.Gen,...
   2,DE,1985,MFH,oil,DE.N.MFH.08.Gen,...
   3,DE,1995,AB,district,DE.N.AB.09.Gen,...

**Import Functions:**

.. code-block:: go

   // Import TABULA data for specific country
   func ImportTabulaData(pool *pgxpool.Pool, config *config.Config) error

   // Generic CSV import with PostgreSQL COPY
   func ImportCsvWithPsql(filePath string, config *config.Config) error

**Import Process:**

1. **File Discovery**: Locate CSV file based on country configuration
2. **Data Validation**: Verify CSV structure and data types
3. **Import Execution**: Use PostgreSQL COPY for efficient bulk import
4. **Constraint Validation**: Verify foreign keys and data integrity
5. **Index Creation**: Create indexes on commonly queried columns

CSV Import Implementation
~~~~~~~~~~~~~~~~~~~~~~~~~

**PostgreSQL COPY Command:**

.. code-block:: go

   func ImportCsvWithPsql(filePath string, config *config.Config) error {
       // Convert to absolute path
       absPath, err := filepath.Abs(filePath)
       if err != nil {
           return fmt.Errorf("failed to get absolute path: %v", err)
       }

       // Build COPY command
       copyCommand := fmt.Sprintf(
           "\\copy tabula.tabula FROM '%s' DELIMITER ',' CSV HEADER",
           absPath)

       // Execute psql command
       cmd := exec.Command("psql",
           "-h", config.DB.Host,
           "-U", config.DB.User,
           "-d", config.DB.Name,
           "-c", copyCommand)

       // Set password environment
       cmd.Env = append(cmd.Env,
                       fmt.Sprintf("PGPASSWORD=%s", config.DB.Password))

       return cmd.Run()
   }

**Error Handling:**

.. code-block:: go

   // Handle common CSV import errors
   if strings.Contains(errorOutput, "duplicate key value") {
       return fmt.Errorf("duplicate data detected: %v", err)
   }

   if strings.Contains(errorOutput, "invalid input syntax") {
       return fmt.Errorf("invalid CSV format: %v", err)
   }

Data Validation
~~~~~~~~~~~~~~~

**Pre-import Validation:**

.. code-block:: go

   // Validate CSV structure before import
   func ValidateCSVStructure(filePath string) error {
       file, err := os.Open(filePath)
       if err != nil {
           return err
       }
       defer file.Close()

       reader := csv.NewReader(file)

       // Read header
       header, err := reader.Read()
       if err != nil {
           return err
       }

       // Validate required columns
       requiredColumns := []string{"id", "country", "building_type", "tabula_class"}
       return validateColumns(header, requiredColumns)
   }


Supplementary Data Import
-------------------------

SQL Script Processing
~~~~~~~~~~~~~~~~~~~~~

The supplementary importer executes SQL scripts for additional data processing:

**Core Functions:**

.. code-block:: go

   // Import supplementary data with SQL pipeline
   func ImportSupplementaryData(pool *pgxpool.Pool, config *config.Config) error

**Processing Pipeline:**

1. **TABULA Data Import**: Import CSV reference data
2. **SQL Script Execution**: Run supplementary SQL scripts
3. **Attribute Extraction**: Extract building attributes from TABULA data
4. **Data Enrichment**: Add calculated fields and relationships

**Pipeline Implementation:**

.. code-block:: go

   func ImportSupplementaryData(pool *pgxpool.Pool, config *config.Config) error {
       // Step 1: Import TABULA CSV data
       if err := ImportTabulaData(pool, config); err != nil {
           return err
       }

       // Step 2: Execute supplementary SQL scripts
       pipelineQueue, err := process.SupplementaryPipelineQueue(config)
       if err != nil {
           return fmt.Errorf("failed to setup pipeline: %w", err)
       }

       // Step 3: Process with workers
       return processSupplementaryPipeline(pipelineQueue, pool, config)
   }

SQL Script Management
~~~~~~~~~~~~~~~~~~~~~

**Script Organization:**

.. code-block:: text

   sql/scripts/supplementary/
   ├── 01_extract_tabula_attributes.sql    # Extract building attributes
   ├── 02_enrich_building_data.sql         # Add calculated fields
   └── 03_create_relationships.sql         # Create data relationships

**Template Processing:**

.. code-block:: sql

   -- Example: sql/scripts/supplementary/01_extract_tabula_attributes.sql
   INSERT INTO {city2tabula_schema}.building_attributes (
       building_id,
       tabula_class,
       construction_year,
       building_type
   )
   SELECT
       b.id,
       t.tabula_class,
       t.year_of_construction,
       t.building_type
   FROM {tabula_schema}.tabula t
   JOIN {city2tabula_schema}.buildings b ON b.country = t.country;

Worker Pool Integration
~~~~~~~~~~~~~~~~~~~~~~~

**Parallel Processing:**

.. code-block:: go

   // Process supplementary pipeline with workers
   func processSupplementaryPipeline(queue *process.PipelineQueue,
                                   pool *pgxpool.Pool,
                                   config *config.Config) error {

       // Create pipeline channel
       pipelineChan := make(chan *process.Pipeline, queue.Len())

       // Enqueue pipelines
       for !queue.IsEmpty() {
           pipeline := queue.Dequeue()
           if pipeline != nil {
               pipelineChan <- pipeline
           }
       }
       close(pipelineChan)

       // Process with single worker (sequential for data integrity)
       numWorkers := 1
       var wg sync.WaitGroup

       for i := 1; i <= numWorkers; i++ {
           wg.Add(1)
           worker := process.NewWorker(i)
           go worker.Start(pipelineChan, pool, &wg, config)
       }

       wg.Wait()
       return nil
   }

Performance Optimization
------------------------

Bulk Import Strategies
~~~~~~~~~~~~~~~~~~~~~~

**PostgreSQL COPY vs INSERT:**

+------------------+------------------+------------------+------------------+
| Method           | Speed            | Memory Usage     | Use Case         |
+==================+==================+==================+==================+
| COPY             | Very Fast        | Low              | Large CSV files  |
+------------------+------------------+------------------+------------------+
| Bulk INSERT      | Fast             | Medium           | Generated data   |
+------------------+------------------+------------------+------------------+
| Single INSERT    | Slow             | Low              | Small datasets   |
+------------------+------------------+------------------+------------------+

**Optimization Techniques:**

.. code-block:: sql

   -- Disable autocommit for faster imports
   BEGIN;

   -- Temporarily drop indexes during import
   DROP INDEX IF EXISTS tabula_country_idx;

   -- Import data
   \\copy tabula.tabula FROM 'data.csv' DELIMITER ',' CSV HEADER;

   -- Recreate indexes
   CREATE INDEX tabula_country_idx ON tabula.tabula(country);

   COMMIT;

Memory Management
~~~~~~~~~~~~~~~~~

**Large File Processing:**

.. code-block:: go

   // Stream large files to avoid memory issues
   func ProcessLargeCSV(filePath string) error {
       file, err := os.Open(filePath)
       if err != nil {
           return err
       }
       defer file.Close()

       reader := csv.NewReader(file)
       reader.ReuseRecord = true  // Reuse slice for memory efficiency

       for {
           record, err := reader.Read()
           if err == io.EOF {
               break
           }
           if err != nil {
               return err
           }

           // Process record
           processRecord(record)
       }

       return nil
   }

Error Recovery
--------------

Import Failure Recovery
~~~~~~~~~~~~~~~~~~~~~~~

**Transaction Management:**

.. code-block:: go

   // Atomic import with rollback capability
   func ImportWithTransaction(pool *pgxpool.Pool,
                             importFunc func(*pgx.Tx) error) error {

       conn, err := pool.Acquire(context.Background())
       if err != nil {
           return err
       }
       defer conn.Release()

       tx, err := conn.Begin(context.Background())
       if err != nil {
           return err
       }
       defer tx.Rollback(context.Background())

       if err := importFunc(tx); err != nil {
           return err  // Automatic rollback
       }

       return tx.Commit(context.Background())
   }

**Checkpoint and Resume:**

.. code-block:: go

   // Resume import from last successful checkpoint
   func ResumeImport(config *config.Config, checkpointFile string) error {
       checkpoint, err := loadCheckpoint(checkpointFile)
       if err != nil {
           return err
       }

       // Resume from last processed file
       return continueImport(config, checkpoint.LastProcessedFile)
   }

Data Integrity Checks
~~~~~~~~~~~~~~~~~~~~~~

**Post-import Validation:**

.. code-block:: go

   // Validate imported data integrity
   func ValidateImportedData(pool *pgxpool.Pool, config *config.Config) error {
       checks := []ValidationCheck{
           validateRecordCounts,
           validateDataTypes,
           validateConstraints,
           validateSpatialData,
       }

       for _, check := range checks {
           if err := check(pool, config); err != nil {
               return fmt.Errorf("validation failed: %w", err)
           }
       }

       return nil
   }

Monitoring and Logging
----------------------

Import Progress Tracking
~~~~~~~~~~~~~~~~~~~~~~~~

**Progress Metrics:**

.. code-block:: go

   type ImportMetrics struct {
       FilesProcessed    int           // Number of files processed
       RecordsImported   int64         // Total records imported
       BytesProcessed    int64         // Total bytes processed
       StartTime         time.Time     // Import start time
       CurrentFile       string        // Currently processing file
       EstimatedETA      time.Duration // Estimated completion time
   }

**Real-time Monitoring:**

.. code-block:: go

   // Monitor import progress
   func MonitorImportProgress(metrics *ImportMetrics) {
       ticker := time.NewTicker(30 * time.Second)
       defer ticker.Stop()

       for range ticker.C {
           elapsed := time.Since(metrics.StartTime)
           rate := float64(metrics.RecordsImported) / elapsed.Seconds()

           utils.Info.Printf("Import Progress: %d files, %d records, %.0f records/sec",
                           metrics.FilesProcessed,
                           metrics.RecordsImported,
                           rate)
       }
   }

Usage Examples
--------------

Complete Data Import
~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   package main

   import (
       "City2TABULA/internal/config"
       "City2TABULA/internal/db"
       "City2TABULA/internal/importer"
   )

   func main() {
       config := config.LoadConfig()

       pool, err := db.ConnectPool(config)
       if err != nil {
           log.Fatalf("Database connection failed: %v", err)
       }
       defer pool.Close()

       // Import CityDB data (LOD2 and LOD3)
       if err := importer.ImportCityDBData(pool, config); err != nil {
           log.Fatalf("CityDB import failed: %v", err)
       }

       // Import TABULA and supplementary data
       if err := importer.ImportSupplementaryData(pool, config); err != nil {
           log.Fatalf("Supplementary import failed: %v", err)
       }

       log.Println("All data imported successfully")
   }

Custom Import Pipeline
~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Custom import with specific error handling
   func CustomImportPipeline(config *config.Config) error {
       pool, err := db.ConnectPool(config)
       if err != nil {
           return err
       }
       defer pool.Close()

       // Import with checkpoints for large datasets
       checkpoint := &ImportCheckpoint{}

       if err := importWithCheckpoints(pool, config, checkpoint); err != nil {
           // Save checkpoint for resume
           checkpoint.Save("import_checkpoint.json")
           return err
       }

       return nil
   }

For more information on configuration and processing, see :doc:`config_module` and :doc:`process_module`.