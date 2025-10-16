Importer Module
===============

The ``internal/importer`` package handles data import operations for CityDB and supplementary data sources.

Overview
--------

**Components:**
- **CityDB Importer**: 3D building data import (LOD2/LOD3)
- **Supplementary Importer**: TABULA reference data and SQL scripts
- **Format Support**: CityGML, CityJSON, and CSV formats
- **Multi-threaded Import**: Parallel processing for large datasets

**Import Workflow:**

.. code-block:: text

   GML/JSON Files → CityDB Tool → Database Schemas
   CSV Files → psql → Reference Tables
   SQL Scripts → Pipeline Processing

CityDB Import (citydb.go)
-------------------------

Imports 3D building data using the CityDB command-line tool:

**Main Function:**

.. code-block:: go

   func ImportCityDBData(conn *pgxpool.Pool, config *config.Config) error

**Key Features:**
- Automatic CityDB executable detection
- Support for both LOD2 and LOD3 data
- CityGML and CityJSON format handling
- Multi-threaded import processing
- Skip mode for existing data

**Import Process:**

1. **Tool Validation**: Verify CityDB executable exists and works
2. **LOD2 Import**: Import LOD2 CityGML and CityJSON files
3. **LOD3 Import**: Import LOD3 CityGML and CityJSON files
4. **Schema Organization**: Data imported to LOD-specific schemas

**Command Generation:**

.. code-block:: go

   cmd := exec.Command(cityDBExecutable,
       "import",
       "--log-level=debug",
       format,                    // "citygml" or "cityjson"
       "--import-mode=skip",      // Skip existing data
       "--threads=N",             // Parallel processing
       "--db-schema=lod2_country", // Target schema
       dataPath)

**Usage:**

.. code-block:: go

   import "City2TABULA/internal/importer"

   err := importer.ImportCityDBData(dbConn, config)
   if err != nil {
       log.Fatalf("CityDB import failed: %v", err)
   }

Supplementary Import (supplementary.go)
---------------------------------------

Handles TABULA reference data and SQL script execution:

**Main Functions:**

.. code-block:: go

   func ImportSupplementaryData(conn *pgxpool.Pool, config *config.Config) error
   func ImportTabulaData(conn *pgxpool.Pool, config *config.Config) error
   func ImportCsvWithPsql(filePath string, config *config.Config) error

**TABULA Data Import:**

- **Source**: Country-specific CSV files (e.g., ``germany.csv``)
- **Target**: ``tabula.tabula`` table
- **Tool**: PostgreSQL ``psql`` with ``\copy`` command
- **Format**: CSV with headers

**CSV Import Command:**

.. code-block:: sql

   \copy tabula.tabula FROM '/path/to/germany.csv' DELIMITER ',' CSV HEADER

**SQL Script Processing:**

- Uses process pipeline for supplementary scripts
- Single-worker execution for data consistency
- Sequential script processing

**Usage:**

.. code-block:: go

   // Import TABULA reference data
   err := importer.ImportTabulaData(dbConn, config)

   // Import all supplementary data (TABULA + SQL scripts)
   err := importer.ImportSupplementaryData(dbConn, config)

Configuration Requirements
--------------------------

**Environment Variables:**

.. code-block:: bash

   # CityDB Tool Configuration
   CITYDB_TOOL_PATH=/path/to/citydb/tools

   # Database Configuration
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=your_password
   DB_NAME=city2tabula

   # Data Configuration
   COUNTRY=germany

**Data Directory Structure:**

.. code-block:: text

   data/
   ├── lod2/
   │   └── germany/           # LOD2 GML/JSON files
   ├── lod3/
   │   └── germany/           # LOD3 GML/JSON files
   └── tabula/
       ├── germany.csv        # Country-specific TABULA data
       ├── austria.csv
       └── netherlands.csv

**Required Tools:**

- **CityDB CLI**: 3D City Database command-line interface
- **PostgreSQL**: psql client for CSV import
- **File System**: Read access to data directories

Import Workflow
--------------

**Complete Import Sequence:**

.. code-block:: go

   import (
       "City2TABULA/internal/importer"
       "City2TABULA/internal/config"
       "City2TABULA/internal/db"
   )

   func RunCompleteImport() error {
       // 1. Load configuration
       cfg := config.LoadConfig()

       // 2. Establish database connection
       conn, err := db.ConnectDB(cfg)
       if err != nil {
           return err
       }
       defer conn.Close()

       // 3. Import CityDB data (LOD2 + LOD3)
       if err := importer.ImportCityDBData(conn, cfg); err != nil {
           return fmt.Errorf("CityDB import failed: %w", err)
       }

       // 4. Import supplementary data (TABULA + scripts)
       if err := importer.ImportSupplementaryData(conn, cfg); err != nil {
           return fmt.Errorf("Supplementary import failed: %w", err)
       }

       return nil
   }

**Schema Creation Order:**

1. Database schemas (via setup pipeline)
2. CityDB infrastructure (via CityDB tool)
3. Building data (LOD2/LOD3 import)
4. Reference data (TABULA CSV import)
5. Supplementary scripts (processing functions)



