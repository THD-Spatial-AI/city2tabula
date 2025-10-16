Configuration Module
==================

The ``internal/config`` package provides a comprehensive configuration management system for City2TABULA. It handles environment variables, database settings, file paths, and various operational parameters required by the application.

Overview
--------

The configuration system is built around a unified ``Config`` struct that aggregates specialized configuration modules:

- **Main Configuration**: Central config management and validation
- **Database Configuration**: Schema and table management for multiple data sources
- **CityDB Configuration**: 3D City Database integration settings
- **Data Paths**: File system path management for different data types
- **Batch Processing**: Threading and retry configuration
- **SQL Management**: Dynamic SQL script loading and parameter management
- **Environment Handling**: Environment variable loading and normalization

Core Components
---------------

Main Configuration (config.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The main configuration entry point that unifies all configuration modules:

.. code-block:: go

   type Config struct {
       Country    string
       DB         *DBConfig
       Data       *DataPaths
       CityDB     *CityDB
       Batch      *BatchConfig
       RetryConfig *RetryConfig
   }

Key Functions:

- ``LoadConfig()``: Loads and validates the complete configuration
- ``getCountry()``: Normalizes country names for consistency
- ``Validate()``: Validates all configuration settings

Database Configuration (database.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Manages database connections, schemas, and table configurations for multiple data sources:

**Schema Management:**

.. code-block:: go

   type DBSchemas struct {
       Public       string  // "public"
       CityDB       string  // "citydb"
       CityDBPkg    string  // "citydb_pkg"
       Lod2         string  // "lod2_<country>"
       Lod3         string  // "lod3_<country>"
       Tabula       string  // "tabula"
       City2Tabula  string  // "city2tabula"
   }

**Table Configuration:**

.. code-block:: go

   type DBTables struct {
       Building      string  // "building"
       Tabula        string  // Country-specific tabula table
       TabulaVariant string  // "tabula_variant"
   }

**Environment Variables:**

- ``DB_HOST``: Database host (default: localhost)
- ``DB_PORT``: Database port (default: 5432)
- ``DB_USER``: Database username (default: postgres)
- ``DB_PASSWORD``: Database password (default: postgres)
- ``DB_NAME``: Database name (default: city2tabula)
- ``COUNTRY``: Target country for processing

CityDB Configuration (citydb.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Handles 3D City Database (CityDB) integration and tooling:

.. code-block:: go

   type CityDB struct {
       SRSName   string    // Spatial Reference System name
       ToolPath  string    // Path to CityDB tools
       SRID      string    // Spatial Reference ID
       LODLevels []int     // Supported Level of Detail levels [2, 3]
       SQLScripts struct {
           CreateDB     string
           CreateSchema string
           DropDB       string
           DropSchema   string
       }
   }

**Environment Variables:**

- ``CITYDB_TOOL_PATH``: Path to CityDB installation
- ``CITYDB_SRS_NAME``: Spatial reference system name
- ``CITYDB_SRID``: Spatial reference identifier

Data Paths Configuration (data.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Manages file system paths for different data types:

.. code-block:: go

   type DataPaths struct {
       Base   string  // "data/"
       Lod2   string  // "data/lod2/<country>/"
       Lod3   string  // "data/lod3/<country>/"
       Tabula string  // "data/tabula/"
   }

The system automatically constructs country-specific paths based on the ``COUNTRY`` environment variable.

Batch Processing Configuration (batch.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Controls parallel processing and retry behavior:

.. code-block:: go

   type BatchConfig struct {
       Size        int  // Batch size for processing
       ThreadCount int  // Number of worker threads
   }

   type RetryConfig struct {
       MaxAttempts int           // Maximum retry attempts
       BaseDelay   time.Duration // Base delay between retries
       MaxDelay    time.Duration // Maximum delay cap
       Multiplier  float64       // Exponential backoff multiplier
   }

**Environment Variables:**

- ``BATCH_SIZE``: Processing batch size (default: 1000)
- ``THREAD_COUNT``: Worker thread count (default: CPU count)

**Intelligent Defaults:**

- Thread count automatically detects CPU cores
- Exponential backoff retry strategy
- Configurable retry limits and delays

SQL Management (sql.go)
~~~~~~~~~~~~~~~~~~~~~~~

Dynamically loads and manages SQL scripts with template parameters:

.. code-block:: go

   type SQLScripts struct {
       MainScripts          []string  // Core feature extraction (01-10)
       SupplementaryScripts []string  // Supporting scripts
       TableScripts         []string  // Schema creation
       FunctionScripts      []string  // Function definitions
   }

   type SQLParameters struct {
       BuildingIDs        []int64
       LodSchema          string
       SRID               string
       City2TabulaSchema  string
       TabulaSchema       string
       LodLevel           int
       PublicSchema       string
       CityDBSchema       string
       CityDBPkgSchema    string
       Country            string
       TabulaTable        string
       TabulaVariantTable string
   }

**Key Features:**

- Automatic SQL file discovery and sorting
- Template parameter generation for SQL scripts
- LOD-specific schema resolution
- Organized script categories (main, supplementary, schema, functions)

Environment Handling (env.go)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

Provides environment variable utilities and normalization functions:

**Key Functions:**

- ``LoadEnv()``: Loads variables from ``.env`` file
- ``GetEnv(key, fallback)``: Gets environment variable with fallback
- ``GetEnvAsInt(key, fallback)``: Gets integer environment variable
- ``normalizeCountryName(name)``: Normalizes country names for consistency

Configuration Usage
-------------------

Basic Setup
~~~~~~~~~~~

.. code-block:: go

   import "path/to/internal/config"

   // Load configuration
   cfg := config.LoadConfig()

   // Access database settings
   dbHost := cfg.DB.Host
   dbPort := cfg.DB.Port

   // Access data paths
   lod2Path := cfg.Data.Lod2
   tabulaPath := cfg.Data.Tabula

   // Get SQL parameters for processing
   params := cfg.GetSQLParameters(2, buildingIDs)

Environment Variables
~~~~~~~~~~~~~~~~~~~~

Create a ``.env`` file in your project root:

.. code-block:: bash

   # Database Configuration
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=your_password
   DB_NAME=city2tabula

   # Processing Configuration
   COUNTRY=germany
   BATCH_SIZE=1000
   THREAD_COUNT=8

   # CityDB Configuration
   CITYDB_TOOL_PATH=/path/to/citydb/tools
   CITYDB_SRS_NAME=EPSG:25832
   CITYDB_SRID=25832

Schema and Table Structure
~~~~~~~~~~~~~~~~~~~~~~~~~

The configuration system manages multiple database schemas:

.. list-table:: Database Schemas
   :header-rows: 1
   :widths: 20 30 50

   * - Schema
     - Purpose
     - Description
   * - ``public``
     - Default PostgreSQL schema
     - Standard database objects
   * - ``citydb``
     - CityDB core schema
     - 3D city database infrastructure
   * - ``citydb_pkg``
     - CityDB packages
     - CityDB stored procedures and functions
   * - ``lod2_<country>``
     - LOD2 data storage
     - Country-specific Level of Detail 2 data
   * - ``lod3_<country>``
     - LOD3 data storage
     - Country-specific Level of Detail 3 data
   * - ``tabula``
     - TABULA reference data
     - Building typology reference tables
   * - ``city2tabula``
     - Processing results
     - Extracted features and analysis results

Country-Specific Configuration
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

The system supports multiple countries with normalized naming:

.. list-table:: Supported Countries
   :header-rows: 1
   :widths: 30 30 40

   * - Country Name
     - Normalized Form
     - Data Paths
   * - ``Germany``
     - ``germany``
     - ``data/lod2/germany/``, ``data/lod3/germany/``
   * - ``Austria``
     - ``austria``
     - ``data/lod2/austria/``, ``data/lod3/austria/``
   * - ``Netherlands``
     - ``netherlands``
     - ``data/lod2/netherlands/``, ``data/lod3/netherlands/``
   * - ``Czech Republic``
     - ``czech``
     - ``data/lod2/czech/``, ``data/lod3/czech/``

SQL Script Management
~~~~~~~~~~~~~~~~~~~~

SQL scripts are automatically discovered and organized:

.. code-block:: go

   // Load all SQL scripts
   scripts, err := cfg.LoadSQLScripts()
   if err != nil {
       log.Fatal(err)
   }

   // Access categorized scripts
   mainScripts := scripts.MainScripts          // Feature extraction pipeline
   setupScripts := scripts.SupplementaryScripts // Setup and utility scripts
   schemaScripts := scripts.TableScripts       // Schema creation
   functionScripts := scripts.FunctionScripts  // Function definitions

**Script Categories:**

- **Main Scripts** (``sql/scripts/main/``): Core feature extraction pipeline (01-10)
- **Supplementary Scripts** (``sql/scripts/supplementary/``): Supporting operations
- **Table Scripts** (``sql/schema/``): Database schema creation
- **Function Scripts** (``sql/functions/``): Custom database functions

Advanced Configuration
---------------------

Custom Validation
~~~~~~~~~~~~~~~~~

The configuration system includes validation for critical settings:

.. code-block:: go

   cfg := config.LoadConfig()
   if err := cfg.Validate(); err != nil {
       log.Fatalf("Configuration validation failed: %v", err)
   }

Retry Configuration
~~~~~~~~~~~~~~~~~~

Customize retry behavior for robust processing:

.. code-block:: go

   // Access retry settings
   maxRetries := cfg.RetryConfig.MaxAttempts
   baseDelay := cfg.RetryConfig.BaseDelay
   maxDelay := cfg.RetryConfig.MaxDelay

Thread Management
~~~~~~~~~~~~~~~~

Optimize parallel processing:

.. code-block:: go

   // Get optimal thread count (automatically detects CPU cores)
   threadCount := cfg.Batch.ThreadCount
   batchSize := cfg.Batch.Size

