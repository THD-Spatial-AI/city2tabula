Configuration Package Documentation
====================================

The ``internal/config`` package provides centralized configuration management for the City2TABULA pipeline, handling environment variables, database connections, file paths, and processing parameters through a well-organized modular structure.

Package Overview
----------------

The configuration system is built around a modular package structure with clear separation of concerns, organizing settings into logical, domain-specific modules:

.. code-block:: text

   internal/config/
   ├── config.go       # Main Config struct and LoadConfig() - central entry point
   ├── database.go     # Database configuration (connection, schemas, tables)
   ├── data.go         # Data directory paths and file organization
   ├── citydb.go       # CityDB tool and 3D spatial database configuration
   ├── batch.go        # Batch processing and retry configuration
   ├── sql.go          # SQL script management and template parameters
   └── env.go          # Environment variable helpers and utilities

The configuration hierarchy organizes settings into logical groups with clear responsibilities:

.. code-block:: text

   Config
   ├── Country          # Target country/region for processing
   ├── DB               # Database configuration and connection settings
   │   ├── Host, Port, User, Password  # Connection parameters
   │   ├── Tables       # Database table name definitions
   │   ├── Schemas      # Database schema organization
   │   └── SQL          # Dynamic SQL script loading
   ├── Data             # File system paths and data organization
   │   ├── Base         # Root data directory
   │   ├── Lod2, Lod3   # Country-specific LOD data paths
   │   └── Tabula       # TABULA reference data directory
   ├── CityDB           # 3D CityDB tool and spatial configuration
   │   ├── ToolPath     # CityDB tool executable location
   │   ├── SRID, SRSName # Spatial reference system settings
   │   └── SQLScripts   # CityDB-specific SQL operations
   └── Batch            # Processing configuration and performance tuning
       ├── Size, Threads      # Batch processing parameters
       ├── DBConnections      # Database connection pool settings
       └── RetryConfig        # Error handling and retry logic

Core Modules
------------

Main Configuration (config.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The central configuration module provides the main entry point and validation:

**Core Structure:**

.. code-block:: go

   // Main Config holds the application configuration
   type Config struct {
       Country     string        // Target country/region
       DB          *DBConfig     // Database configuration
       Data        *DataPaths    # Data directory paths
       CityDB      *CityDB       # CityDB tool configuration
       Batch       *BatchConfig  # Batch processing settings
       RetryConfig *RetryConfig  # Retry and error handling
   }

**Single Entry Point:**

.. code-block:: go

   // LoadConfig is the single entry point for all configuration
   func LoadConfig() Config {
       LoadEnv()  // Load environment variables
       
       return Config{
           Country:     getCountry(),
           DB:          loadDBConfig(),
           Data:        loadDataPaths(),
           CityDB:      loadCityDBConfig(),
           Batch:       loadBatchConfig(),
           RetryConfig: DefaultRetryConfig(),
       }
   }

**Configuration Validation:**

.. code-block:: go

   // Validate checks if the configuration is valid
   func (c Config) Validate() error {
       // Validates all configuration components
       // Returns detailed error messages for missing or invalid settings
   }

Database Configuration (database.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Manages all database-related configuration including connections, schemas, and table definitions:

**Database Structure:**

.. code-block:: go

   // Database configuration
   type DBConfig struct {
       Host     string     // Database hostname
       Port     string     # Database port
       Name     string     # Database name (dynamically includes country)
       User     string     # Database username
       Password string     # Database password
       SSLMode  string     # SSL connection mode
       
       Tables  *Tables     # Database table definitions
       Schemas *Schemas    # Database schema organization
       SQL     *SQLScripts # Dynamic SQL script loading
   }

**Schema Organization:**

.. code-block:: go

   type Schemas struct {
       Public    string    // public schema
       CityDB    string    // citydb - 3D CityDB core schema
       CityDBPkg string    // citydb_pkg - CityDB packages
       Lod2      string    // lod2 - Level of Detail 2 buildings
       Lod3      string    // lod3 - Level of Detail 3 buildings
       Tabula    string    // tabula - TABULA building classifications
       Training  string    // training - feature extraction results
   }

**Table Definitions:**

.. code-block:: go

   type Tables struct {
       TrainingData  string    // training_data - final ML dataset
       Tabula        string    // tabula - building type classifications
       TabulaVariant string    // tabula_variant - building subtypes
   }

Data Path Configuration (data.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Organizes file system paths and data directory structure:

**Data Organization:**

.. code-block:: go

   // Data directory constants
   const (
       DataDir       = "data/"
       Lod2DataDir   = DataDir + "lod2/"
       Lod3DataDir   = DataDir + "lod3/"
       TabulaDataDir = DataDir + "tabula/"
   )

   // Data paths with country-specific organization
   type DataPaths struct {
       Base   string    // data/ - root data directory
       Lod2   string    // data/lod2/{country}/ - LOD2 CityGML files
       Lod3   string    # data/lod3/{country}/ - LOD3 CityGML files
       Tabula string    # data/tabula/ - TABULA CSV files
   }

**Country-Specific Paths:**

.. code-block:: go

   func loadDataPaths() *DataPaths {
       country := normalizeCountryName(GetEnv("COUNTRY", ""))
       return &DataPaths{
           Base:   DataDir,
           Lod2:   Lod2DataDir + country,  // e.g., data/lod2/germany/
           Lod3:   Lod3DataDir + country,  // e.g., data/lod3/germany/
           Tabula: TabulaDataDir,          // data/tabula/
       }
   }

CityDB Configuration (citydb.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Manages 3D CityDB tool configuration and spatial reference systems:

**CityDB Structure:**

.. code-block:: go

   type CityDB struct {
       SRSName   string    // Spatial Reference System name
       ToolPath  string    # Path to citydb-tool executable
       SRID      string    # Spatial Reference ID (e.g., "25832")
       LODLevels []int     # Supported LOD levels [2, 3]
       
       // CityDB-specific SQL operations
       SQLScripts struct {
           CreateDB     string    # Database creation script
           CreateSchema string    # Schema creation script
           DropDB       string    # Database cleanup script
           DropSchema   string    # Schema cleanup script
       }
   }

**Spatial Reference Configuration:**

.. code-block:: go

   func loadCityDBConfig() *CityDB {
       return &CityDB{
           SRSName:   GetEnv("CITYDB_SRS_NAME", "ETRS89 / UTM zone 32N"),
           ToolPath:  GetEnv("CITYDB_TOOL_PATH", "citydb-tool"),
           SRID:      GetEnv("CITYDB_SRID", "25832"),
           LODLevels: []int{2, 3},  // LOD2 and LOD3 support
       }
   }

Batch Processing Configuration (batch.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Handles batch processing, performance tuning, and retry logic:

**Batch Configuration:**

.. code-block:: go

   type BatchConfig struct {
       Size           int    // Buildings per batch
       Threads        int    // Parallel worker threads
       DBMaxOpenConns int    // Maximum database connections
       DBMaxIdleConns int    // Idle database connections
   }

**Retry Configuration:**

.. code-block:: go

   type RetryConfig struct {
       MaxRetries      int           // Maximum retry attempts
       InitialDelay    time.Duration // Initial retry delay
       MaxDelay        time.Duration // Maximum retry delay
       BackoffFactor   float64       // Exponential backoff multiplier
       DeadlockRetries int          # Special handling for deadlocks
   }

**Intelligent Defaults:**

.. code-block:: go

   func loadBatchConfig() *BatchConfig {
       return &BatchConfig{
           Size:           getEnvAsInt("BATCH_SIZE", 1000),
           Threads:        getEnvAsInt("BATCH_THREADS", runtime.NumCPU()),
           DBMaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
           DBMaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
       }
   }

SQL Script Management (sql.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Manages SQL script organization and template parameters:

**SQL Script Categories:**

.. code-block:: go

   type SQLScripts struct {
       MainScripts          []string  // Core feature extraction (01-10)
       SupplementaryScripts []string  // Supporting scripts (tabula, etc.)
       TableScripts         []string  // Schema creation scripts
       FunctionScripts      []string  // Custom function definitions
   }

**SQL Directory Organization:**

.. code-block:: go

   const (
       SQLDir = "sql/"
       
       // Script categories
       SQLMainScriptDir          = SQLDir + "scripts/main/"
       SQLSupplementaryScriptDir = SQLDir + "scripts/supplementary/"
       SQLSchemaFileDir          = SQLDir + "schema/"
       SQLTrainingFunctionsPath  = SQLDir + "functions/"
   )

**Template Parameters:**

.. code-block:: go

   type SQLParameters struct {
       BuildingIDs        []int64  `param:"building_ids"`
       LodSchema          string   `param:"lod_schema"`
       TrainingSchema     string   `param:"city2tabula_schema"`
       TabulaSchema       string   `param:"tabula_schema"`
       SRID               string   `param:"srid"`
       Country            string   `param:"country"`
       // ... additional parameters
   }

Environment Configuration (env.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Provides environment variable handling and utilities:

**Environment Utilities:**

.. code-block:: go

   // GetEnv gets environment variable with default fallback
   func GetEnv(key, defaultValue string) string

   // getEnvAsInt converts environment variable to integer
   func getEnvAsInt(key string, defaultValue int) int

   // LoadEnv loads environment variables from .env file
   func LoadEnv()

Configuration Usage Patterns
-----------------------------

Basic Configuration Loading
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   package main

   import (
       "City2TABULA/internal/config"
       "City2TABULA/internal/utils"
   )

   func main() {
       // Load complete configuration
       cfg := config.LoadConfig()
       
       // Validate configuration
       if err := cfg.Validate(); err != nil {
           utils.Error.Fatalf("Invalid configuration: %v", err)
       }
       
       // Access configuration components
       utils.Info.Printf("Processing country: %s", cfg.Country)
       utils.Info.Printf("Database: %s", cfg.DB.Name)
       utils.Info.Printf("Batch size: %d", cfg.Batch.Size)
   }

Database Configuration Usage
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Access database configuration
   func connectToDatabase(cfg config.Config) {
       dbConfig := cfg.DB
       
       // Build connection string
       connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
           dbConfig.Host, dbConfig.Port, dbConfig.User, 
           dbConfig.Password, dbConfig.Name, dbConfig.SSLMode)
       
       // Access schema names
       trainingSchema := dbConfig.Schemas.Training  // "training"
       tabulaSchema := dbConfig.Schemas.Tabula      // "tabula"
       lod2Schema := dbConfig.Schemas.Lod2          // "lod2"
       
       // Access table names
       trainingTable := dbConfig.Tables.TrainingData  // "training_data"
   }

Data Path Usage
~~~~~~~~~~~~~~~

.. code-block:: go

   // Access data paths
   func processDataFiles(cfg config.Config) {
       dataPaths := cfg.Data
       
       // Country-specific LOD2 data directory
       lod2Dir := dataPaths.Lod2  // e.g., "data/lod2/germany/"
       
       // List CityGML files
       gmlFiles, err := filepath.Glob(lod2Dir + "*.gml")
       if err != nil {
           utils.Error.Printf("Failed to find GML files in %s: %v", lod2Dir, err)
           return
       }
       
       // Process TABULA data
       tabulaFile := dataPaths.Tabula + cfg.Country + ".csv"
       utils.Info.Printf("TABULA file: %s", tabulaFile)
   }

CityDB Configuration Usage
~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Configure CityDB tool execution
   func importCityDBData(cfg config.Config) error {
       cityDB := cfg.CityDB
       
       // Build CityDB tool command
       cmd := exec.Command(cityDB.ToolPath, "import",
           "--db-host", cfg.DB.Host,
           "--db-port", cfg.DB.Port,
           "--db-name", cfg.DB.Name,
           "--db-schema", cfg.DB.Schemas.Lod2,
           "--srid", cityDB.SRID,
           "--srs-name", cityDB.SRSName,
           "--input-file", "building_data.gml")
       
       return cmd.Run()
   }

Batch Processing Configuration
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Configure batch processing
   func setupBatchProcessing(cfg config.Config) {
       batchConfig := cfg.Batch
       retryConfig := cfg.RetryConfig
       
       // Set up worker pool
       numWorkers := batchConfig.Threads
       batchSize := batchConfig.Size
       
       utils.Info.Printf("Starting %d workers with batch size %d", 
                        numWorkers, batchSize)
       
       // Configure database connection pool
       dbConfig := &pgxpool.Config{
           MaxConns:        int32(batchConfig.DBMaxOpenConns),
           MinConns:        int32(batchConfig.DBMaxIdleConns),
           MaxConnLifetime: time.Hour,
       }
       
       // Configure retry logic
       runner := &process.Runner{
           MaxRetries:    retryConfig.MaxRetries,
           InitialDelay:  retryConfig.InitialDelay,
           BackoffFactor: retryConfig.BackoffFactor,
       }
   }

SQL Template Usage
~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Use SQL template parameters
   func executeSQL(cfg config.Config, buildingIDs []int64, lodLevel int) {
       // Get SQL parameters for specific LOD
       params := cfg.GetSQLParameters(buildingIDs, lodLevel)
       
       // Template SQL with parameters
       sqlTemplate := `
           SELECT building_id, surface_area 
           FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
           WHERE building_id IN {building_ids}
           AND srid = {srid};
       `
       
       // Replace template parameters
       finalSQL := replaceTemplateParameters(sqlTemplate, params)
       
       // Execute SQL
       executeQuery(finalSQL)
   }

Environment Configuration
-------------------------

Environment Variables
~~~~~~~~~~~~~~~~~~~~~

The configuration system supports the following environment variables:

**Database Configuration:**

.. code-block:: bash

   # Database connection
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=your_password
   DB_SSL_MODE=disable

**Application Settings:**

.. code-block:: bash

   # Target country/region
   COUNTRY=germany

   # CityDB tool configuration
   CITYDB_TOOL_PATH=/opt/citydb-tool/citydb-tool
   CITYDB_SRID=25832
   CITYDB_SRS_NAME="ETRS89 / UTM zone 32N"

**Performance Tuning:**

.. code-block:: bash

   # Batch processing
   BATCH_SIZE=1000
   BATCH_THREADS=8

   # Database connections
   DB_MAX_OPEN_CONNS=25
   DB_MAX_IDLE_CONNS=5

**Development Settings:**

.. code-block:: bash

   # Logging
   LOG_LEVEL=INFO

Environment File (.env)
~~~~~~~~~~~~~~~~~~~~~~~

Create a `.env` file in your project root:

.. code-block:: bash

   # City2TABULA Configuration

   # Database Configuration
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=City2TABULA
   DB_PASSWORD=secure_password
   DB_SSL_MODE=disable

   # Processing Configuration
   COUNTRY=germany
   BATCH_SIZE=1000
   BATCH_THREADS=8

   # CityDB Configuration
   CITYDB_TOOL_PATH=/opt/citydb-tool/citydb-tool
   CITYDB_SRID=25832
   CITYDB_SRS_NAME=ETRS89 / UTM zone 32N

   # Performance Tuning
   DB_MAX_OPEN_CONNS=25
   DB_MAX_IDLE_CONNS=5

   # Development
   LOG_LEVEL=INFO

Configuration Validation
------------------------

Validation Rules
~~~~~~~~~~~~~~~~

The configuration system includes comprehensive validation:

.. code-block:: go

   func (c Config) Validate() error {
       var missing []string
       
       // Required fields validation
       if c.Country == "" {
           missing = append(missing, "COUNTRY")
       }
       if c.DB.Host == "" {
           missing = append(missing, "DB_HOST")
       }
       if c.DB.User == "" {
           missing = append(missing, "DB_USER")
       }
       if c.DB.Password == "" {
           missing = append(missing, "DB_PASSWORD")
       }
       
       // Validate ranges
       if c.Batch.Size <= 0 || c.Batch.Size > 10000 {
           return fmt.Errorf("BATCH_SIZE must be between 1 and 10000")
       }
       if c.Batch.Threads <= 0 || c.Batch.Threads > 100 {
           return fmt.Errorf("BATCH_THREADS must be between 1 and 100")
       }
       
       // Check file paths exist
       if c.CityDB.ToolPath != "" {
           if _, err := os.Stat(c.CityDB.ToolPath); os.IsNotExist(err) {
               return fmt.Errorf("CityDB tool not found at: %s", c.CityDB.ToolPath)
           }
       }
       
       if len(missing) > 0 {
           return fmt.Errorf("missing required environment variables: %v", missing)
       }
       
       return nil
   }

Configuration Best Practices
----------------------------

Development vs Production
~~~~~~~~~~~~~~~~~~~~~~~~~

**Development Configuration:**

.. code-block:: bash

   # .env.development
   BATCH_SIZE=100          # Smaller batches for testing
   BATCH_THREADS=2         # Fewer threads for debugging
   LOG_LEVEL=DEBUG         # Verbose logging
   DB_MAX_OPEN_CONNS=5     # Fewer connections

**Production Configuration:**

.. code-block:: bash

   # .env.production
   BATCH_SIZE=5000         # Larger batches for throughput
   BATCH_THREADS=16        # More threads for performance
   LOG_LEVEL=INFO          # Standard logging
   DB_MAX_OPEN_CONNS=50    # More connections

Multi-Environment Setup
~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # Load environment-specific configuration
   ENV=development City2TABULA --extract_features
   ENV=production City2TABULA --extract_features

Security Considerations
~~~~~~~~~~~~~~~~~~~~~~~

- **Never commit `.env` files** with passwords to version control
- **Use environment-specific** configuration files
- **Rotate database passwords** regularly
- **Use SSL connections** in production (`DB_SSL_MODE=require`)

Configuration Testing
~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Test configuration loading
   func TestConfigurationLoading(t *testing.T) {
       // Set test environment variables
       os.Setenv("COUNTRY", "test")
       os.Setenv("DB_HOST", "localhost")
       os.Setenv("DB_USER", "test")
       os.Setenv("DB_PASSWORD", "test")
       
       // Load configuration
       cfg := config.LoadConfig()
       
       // Validate
       if err := cfg.Validate(); err != nil {
           t.Errorf("Configuration validation failed: %v", err)
       }
       
       // Check values
       assert.Equal(t, "test", cfg.Country)
       assert.Equal(t, "localhost", cfg.DB.Host)
   }

For more information on database operations and processing, see :doc:`database_module` and :doc:`process_module`.
```

**constants.go**
    All constants including table names, schema names, and file paths

**database.go**
    Database-related configuration loaders (DB, Tables, Schemas)

**sql.go**
    SQL script paths and parameter handling

**batch.go**
    Batch processing and performance configuration (threads, batch sizes)

**env.go**
    Environment variable helpers (GetEnv, GetEnvAsInt, etc.)

**validation.go**
    Configuration validation logic

**Benefits of Modular Structure**:

- **Single Responsibility**: Each file has a clear, focused purpose
- **Easy Maintenance**: Find and modify specific configurations quickly
- **Better Organization**: Related functionality grouped together
- **Scalable**: Add new config areas without bloating files
- **Testable**: Test individual components in isolation

Main Configuration
~~~~~~~~~~~~~~~~~~

Config
^^^^^^

.. code-block:: go

   type Config struct {
       Country string          // Target country/region
       DB      *DBConfig       // Database configuration (includes Tables, Schemas, SQL)
       Data    *DataPaths      // File system paths
       CityDB  *CityDB         // CityDB configuration
       Batch   *BatchConfig    // Processing parameters
   }

**Purpose**: Main configuration container that holds all application settings with improved hierarchical organization.

**New API Structure**: Database-related configurations are logically grouped under `DB`:

.. code-block:: go

   // Improved: Database configurations under DB
   config.DB.Tables.Lod2ChildFeature     // Table names
   config.DB.Schemas.Training            // Schema names
   config.DB.SQL.ChildFeatureFile        // SQL script paths

   // Other top-level configurations
   config.Country                        // Global settings
   config.Data.Lod2                     // File paths
   config.CityDB.SRID                   // CityDB settings
   config.Batch.Threads                 // Processing settings

**Usage**: Loaded once at application startup and passed throughout the pipeline.

**Example**:

.. code-block:: go

   import "City2TABULA/internal/config"

   config := config.LoadConfig()
   if err := config.Validate(); err != nil {
       log.Fatal("Configuration error:", err)
   }

Database Configuration
~~~~~~~~~~~~~~~~~~~~~~

DBConfig
^^^^^^^^

.. code-block:: go

   type DBConfig struct {
       Host     string
       Port     string
       Name     string
       User     string
       Password string
       SSLMode  string

       // Database structure
       Tables  *Tables        // Database table names
       Schemas *Schemas       // Database schema names
       SQL     *SQLScripts    // SQL script paths
   }

**Purpose**: PostgreSQL database connection parameters.

**Environment Variables**:

- ``DB_HOST`` (default: "localhost")
- ``DB_PORT`` (default: "5432")
- ``DB_USER`` (default: "postgres")
- ``DB_PASSWORD`` (required)
- ``DB_SSL_MODE`` (optional)

**Database Name**: Auto-generated as ``City2TABULA_{country}`` based on ``COUNTRY`` environment variable.

**Example**:

.. code-block:: bash

   # .env file
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=mypassword
   COUNTRY=germany
   # Results in database name: City2TABULA_germany

File System Paths
~~~~~~~~~~~~~~~~~

DataPaths
^^^^^^^^^

.. code-block:: go

   type DataPaths struct {
       Base     string    // "data/"
       Lod2     string    // "data/lod2/"
       Lod3     string    // "data/lod3/"
       Tabula   string    // "data/tabula/"
       Postcode string    // "data/postcode/"
   }

**Purpose**: Defines standardized paths for input data files.

**Directory Structure**:

.. code-block:: text

   data/
   ├── lod2/           # LOD2 CityGML files
   ├── lod3/           # LOD3 CityGML files
   ├── tabula/         # Building type CSV files
   └── postcode/       # Postal code shapefiles

**Usage**:

.. code-block:: go

   tabulaFiles := filepath.Join(config.Data.Tabula, "*.csv")
   postcodeShp := filepath.Join(config.Data.Postcode, country, "*.shp")

CityDB Configuration
~~~~~~~~~~~~~~~~~~~~

CityDB
^^^^^^

.. code-block:: go

   type CityDB struct {
       SRSName   string    // Spatial Reference System name
       ToolPath  string    // Path to 3DCityDB tools
       CRS       string    // Coordinate Reference System (EPSG code)
       LODLevels []int     // Supported Level of Detail values [2, 3]
   }

**Purpose**: Configuration for 3DCityDB tool integration and spatial reference systems.

**Environment Variables**:

- ``CITYDB_TOOL_PATH`` (required): Path to 3DCityDB installation
- ``CITYDB_CRS`` (required): EPSG code (e.g., "EPSG:25832")
- ``CITYDB_SRS_NAME`` (required): Human-readable SRS name

**Example**:

.. code-block:: bash

   CITYDB_TOOL_PATH=/opt/citydb-tool-1.0.0
   CITYDB_CRS=EPSG:25832
   CITYDB_SRS_NAME="ETRS89 / UTM zone 32N"

**Usage**:

.. code-block:: go

   srid := parseSRID(config.CityDB.CRS)  // Extract numeric SRID
   toolPath := config.CityDB.ToolPath   // Access 3DCityDB tools

SQL Scripts Configuration
~~~~~~~~~~~~~~~~~~~~~~~~~

SQLScripts
^^^^^^^^^^

.. code-block:: go

   type SQLScripts struct {
       ChildFeatureFile      SQLPath
       GeomDumpFile          SQLPath
       FeatureAttributesFile SQLPath
       BuildingFeatureFile   SQLPath
       PopulationDensityFile SQLPath
       VolumeCalcFile        SQLPath
       StoreyCalcFile        SQLPath
       NeighbourCalcFile     SQLPath
       TabulaExtractFile     SQLPath
       TrainingTablesFile    SQLPath
       TabulaTablesFile      SQLPath
   }

**Purpose**: Centralized paths to all SQL template files used in the pipeline.

**File Mapping**:

.. code-block:: text

   sql/
   ├── scripts/
   │   ├── citydb/
   │   │   ├── 01_get_child_feat.sql          → ChildFeatureFile
   │   │   ├── 02_dump_child_feat_geom.sql    → GeomDumpFile
   │   │   ├── 03_calc_child_feat_attr.sql    → FeatureAttributesFile
   │   │   ├── 04_calc_bld_feat.sql           → BuildingFeatureFile
   │   │   ├── 05_calc_population_density.sql → PopulationDensityFile
   │   │   ├── 06_calc_volume.sql             → VolumeCalcFile
   │   │   ├── 07_calc_storeys.sql            → StoreyCalcFile
   │   │   └── 08_calc_attached_neighbours.sql → NeighbourCalcFile
   │   └── tabula/
   │       └── extract_tabula_attributes.sql  → TabulaExtractFile
   └── schema/
       ├── create_training_tables.sql         → TrainingTablesFile
       └── create_tabula_tables.sql           → TabulaTablesFile

**Usage**:

.. code-block:: go

   sqlFile := &SQLFile{
       Path:           config.DB.SQL.ChildFeatureFile,
       RequiredParams: []string{"lod_schema", "building_ids"},
   }

Database Schema and Table Names
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Schemas
^^^^^^^

.. code-block:: go

   type Schemas struct {
       Public    string    // "public"
       CityDB    string    // "citydb"
       CityDBPkg string    // "citydb_pkg"
       Lod2      string    // "lod2"
       Lod3      string    // "lod3"
       Tabula    string    // "tabula"
       Training  string    // "training"
   }

Tables
^^^^^^

.. code-block:: go

   type Tables struct {
       Lod2ChildFeature         string    // "lod2_child_feature"
       Lod2ChildFeatureGeomDump string    // "lod2_child_feature_geom_dump"
       Lod2ChildFeatureSurface  string    // "lod2_child_feature_surface"
       Lod2BuildingFeature      string    // "lod2_building_feature"

       Lod3ChildFeature         string    // "lod3_child_feature"
       Lod3ChildFeatureGeomDump string    // "lod3_child_feature_geom_dump"
       Lod3ChildFeatureSurface  string    // "lod3_child_feature_surface"
       Lod3BuildingFeature      string    // "lod3_building_feature"

       Postcode     string    // "postcode"
       TrainingData string    // "training_data"
   }

**Purpose**: Standardized database object names used throughout the application.

**Benefits**:

- **Consistency**: Single source of truth for all database names
- **Maintainability**: Easy to change names without code modification
- **Parameter Substitution**: Used in SQL template replacement

**Usage**:

.. code-block:: go

   tableName := config.DB.Tables.Lod2BuildingFeature  // "lod2_building_feature"
   schemaName := config.DB.Schemas.Training           // "training"
   fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)

Processing Configuration
~~~~~~~~~~~~~~~~~~~~~~~~

BatchConfig
^^^^^^^^^^^

.. code-block:: go

   type BatchConfig struct {
       Size           int    // Buildings per batch (default: 1000)
       Threads        int    // Concurrent workers
       DBMaxOpenConns int    // Max database connections (default: 10)
       DBMaxIdleConns int    // Idle database connections (default: 5)
   }

**Purpose**: Controls performance and resource usage during feature extraction.

**Environment Variables**:

- ``THREAD_COUNT``: Override automatic CPU detection
- ``DB_MAX_OPEN_CONNS``: Maximum database connections
- ``DB_MAX_IDLE_CONNS``: Idle connection pool size

**Thread Count Logic**:

.. code-block:: go

   func getThreadCount() int {
       numCPU := max(runtime.NumCPU(), 1)

       if envThreads := GetEnv("THREAD_COUNT", ""); envThreads != "" {
           // Custom thread count with validation
           if t, err := strconv.Atoi(envThreads); err == nil {
               return min(max(t, 1), numCPU)  // Clamp to [1, numCPU]
           }
       }

       return numCPU  // Use all available CPUs
   }

**Usage**:

.. code-block:: go

   batches := utils.CreateBatches(buildingIDs, config.Batch.Size)
   numWorkers := config.Batch.Threads

   // Database connection pool
   pool, err := pgxpool.New(ctx, connString)
   pool.Config().MaxConns = config.Batch.DBMaxOpenConns

Core Functions
--------------

LoadConfig() Config
~~~~~~~~~~~~~~~~~~~

**Purpose**: Loads and initializes the complete application configuration.

**Package Import**:

.. code-block:: go

   import "City2TABULA/internal/config"

**Process**:

1. Loads ``.env`` file from project root
2. Reads environment variables with fallback defaults
3. Constructs all configuration structures
4. Auto-generates database name from country
5. Validates thread count against available CPUs

**Example**:

.. code-block:: go

   config := config.LoadConfig()
   // Auto-generated based on environment variables
   // No parameters required

(c Config) Validate() error
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Purpose**: Validates that all required configuration is present and valid.

**Validation Rules**:

**Required Environment Variables**: Must be non-empty strings

- ``DB_HOST``, ``DB_PORT``, ``DB_USER``, ``DB_PASSWORD``
- ``CITYDB_TOOL_PATH``, ``CITYDB_CRS``, ``CITYDB_SRS_NAME``
- ``COUNTRY``

**Returns**: Error with list of missing variables, or nil if valid.

**Example**:

.. code-block:: go

   config := config.LoadConfig()
   if err := config.Validate(); err != nil {
       log.Fatal("Configuration error:", err)
   }
   // Validation error: missing required environment variables: DB_PASSWORD, CITYDB_TOOL_PATH

LoadEnv()
~~~~~~~~~

**Purpose**: Loads environment variables from ``.env`` file in project root.

**Behavior**:

- Silently fails if ``.env`` file doesn't exist
- Environment variables override ``.env`` file values
- Uses ``github.com/joho/godotenv`` for parsing

GetEnv(key string, fallback string) string
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Purpose**: Retrieves environment variable with fallback default.

**Parameters**:

- ``key``: Environment variable name
- ``fallback``: Default value if variable is empty/missing

**Returns**: Environment variable value or fallback

GetEnvAsInt(key string, fallback int) int
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Purpose**: Retrieves environment variable as integer with fallback.

**Parameters**:

- ``key``: Environment variable name
- ``fallback``: Default integer value

**Returns**: Parsed integer or fallback if parsing fails

Environment Variables Reference
-------------------------------

Required Variables
~~~~~~~~~~~~~~~~~~

.. list-table::
   :header-rows: 1
   :widths: 25 35 40

   * - Variable
     - Purpose
     - Example
   * - ``DB_PASSWORD``
     - PostgreSQL password
     - ``mypassword123``
   * - ``CITYDB_TOOL_PATH``
     - 3DCityDB installation path
     - ``/opt/3dcitydb``
   * - ``CITYDB_CRS``
     - Coordinate reference system
     - ``EPSG:25832``
   * - ``CITYDB_SRS_NAME``
     - SRS human name
     - ``"ETRS89 / UTM zone 32N"``
   * - ``COUNTRY``
     - Target country/region
     - ``germany``, ``netherlands``

Optional Variables (with defaults)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. list-table::
   :header-rows: 1
   :widths: 25 20 55

   * - Variable
     - Default
     - Purpose
   * - ``DB_HOST``
     - ``localhost``
     - PostgreSQL host
   * - ``DB_PORT``
     - ``5432``
     - PostgreSQL port
   * - ``DB_USER``
     - ``postgres``
     - PostgreSQL username
   * - ``DB_SSL_MODE``
     - ``""``
     - SSL connection mode
   * - ``THREAD_COUNT``
     - ``runtime.NumCPU()``
     - Processing threads
   * - ``DB_MAX_OPEN_CONNS``
     - ``10``
     - Max database connections
   * - ``DB_MAX_IDLE_CONNS``
     - ``5``
     - Idle connection pool

Configuration Examples
----------------------

Development Environment
~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # .env file for local development
   COUNTRY=germany
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=dev123
   DB_SSL_MODE=disable
   CITYDB_TOOL_PATH=/home/user/3dcitydb
   CITYDB_CRS=EPSG:25832
   CITYDB_SRS_NAME="ETRS89 / UTM zone 32N"
   THREAD_COUNT=4

Production Environment
~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # Production environment variables
   COUNTRY=netherlands
   DB_HOST=db.example.com
   DB_PORT=5432
   DB_USER=City2TABULA_prod
   DB_PASSWORD=securepassword
   DB_SSL_MODE=require
   CITYDB_TOOL_PATH=/opt/3dcitydb
   CITYDB_CRS=EPSG:28992
   CITYDB_SRS_NAME="Amersfoort / RD New"
   THREAD_COUNT=16
   DB_MAX_OPEN_CONNS=20
   DB_MAX_IDLE_CONNS=10

Docker Environment
~~~~~~~~~~~~~~~~~~

.. code-block:: yaml

   # docker-compose.yml
   environment:
     - COUNTRY=germany
     - DB_HOST=postgres
     - DB_PASSWORD=${DB_PASSWORD}
     - CITYDB_TOOL_PATH=/app/3dcitydb
     - CITYDB_CRS=EPSG:25832
     - CITYDB_SRS_NAME=ETRS89 / UTM zone 32N

Integration with Pipeline
-------------------------

Database Setup Phase
~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   config := config.LoadConfig()

   // Database connection using config
   pool, err := db.ConnectPool(config)

   // Schema creation using config names
   err = db.CreateSchema(pool, config.DB.Schemas.Training)

Feature Extraction Phase
~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Batch processing using config
   batches := utils.CreateBatches(buildingIDs, config.Batch.Size)

   // SQL execution using config paths
   sqlFile := &SQLFile{
       Path: config.DB.SQL.ChildFeatureFile,
       RequiredParams: []string{"lod_schema", "building_ids"},
   }

   // Worker coordination using config
   numWorkers := config.Batch.Threads

SQL Parameter Substitution
~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Schema and table names from config
   params := map[string]any{
       "city2tabula_schema": config.DB.Schemas.Training,
       "lod_schema":      config.DB.Schemas.Lod2,
       "table_name":      config.Tables.Lod2ChildFeature,
   }

Best Practices
--------------

Configuration Management
~~~~~~~~~~~~~~~~~~~~~~~~

1. **Environment-Specific**: Use different ``.env`` files for dev/staging/prod
2. **Validation**: Always call ``config.Validate()`` at startup
3. **Immutable**: Load configuration once, don't modify during runtime
4. **Secrets**: Use environment variables for sensitive data

Performance Tuning
~~~~~~~~~~~~~~~~~~~

1. **Thread Count**: Start with ``runtime.NumCPU()``, adjust based on I/O vs CPU usage
2. **Batch Size**: 1000 is optimal for most datasets, increase for memory-rich systems
3. **Database Connections**: Balance between concurrency and resource usage

Deployment
~~~~~~~~~~

1. **Container-Ready**: All configuration via environment variables
2. **Country-Specific**: Database names auto-generated per country
3. **Path Flexibility**: Relative paths work in any deployment environment

The configuration module provides a robust, flexible foundation for the entire City2TABULA pipeline, ensuring consistent behavior across different environments and deployment scenarios.

Migration from Previous Version
-------------------------------

The configuration has been refactored from a single large file to a modular package structure with improved API organization:

**Package Import Change**:

.. code-block:: go

   // Old import
   import "City2TABULA/internal/utils"
   config := utils.LoadConfig()

   // New import
   import "City2TABULA/internal/config"
   config := config.LoadConfig()

**API Structure Changes**:

.. list-table::
   :header-rows: 1
   :widths: 50 50

   * - Old API
     - New API (Improved)
   * - ``config.Tables.Lod2ChildFeature``
     - ``config.DB.Tables.Lod2ChildFeature``
   * - ``config.Schemas.Training``
     - ``config.DB.Schemas.Training``
   * - ``config.SQL.ChildFeatureFile``
     - ``config.DB.SQL.ChildFeatureFile``
   * - ``config.DB.Host``
     - ``config.DB.Host`` *(unchanged)*
   * - ``config.Data.Lod2``
     - ``config.Data.Lod2`` *(unchanged)*

**Benefits of New Structure**:

- **Logical Grouping**: Database-related configs under ``config.DB``
- **Modular Files**: Easier to maintain and understand
- **Better API**: More intuitive structure (``DB.Tables`` vs just ``Tables``)
- **Single Import**: Only need ``internal/config`` package
