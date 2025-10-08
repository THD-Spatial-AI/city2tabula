City2TABULA Pipeline Workflow
============================

This document provides a comprehensive overview of the City2TABULA data processing pipeline, from database setup to feature extraction and training data preparation.

Complete Pipeline Overview
--------------------------

The City2TABULA pipeline consists of several database management operations and feature extraction:

1. **Database Creation** (``--create_db``): Create complete database from scratch
2. **Database Reset Options**: Multiple reset strategies for different scenarios
3. **Feature Extraction** (``--extract_features``): Process building data and extract features

Available Commands
------------------

The City2TABULA application provides clear, purpose-specific commands:

**Database Creation:**

- ``--create_db``: Create the complete City2TABULA database (CityDB infrastructure + schemas + data import)

**Database Reset Options:**

- ``--reset_all``: Reset everything - drop all schemas and recreate the complete database
- ``--reset_citydb``: Reset only CityDB infrastructure (drop CityDB schemas, recreate them, and re-import CityDB data)
- ``--reset_city2tabula``: Reset only City2TABULA schemas (preserve CityDB)
- ``--reset_db``: Legacy command - same as ``--reset_all``

**Feature Processing:**

- ``--extract_features``: Run the feature extraction pipeline

Stage 1: Database Creation (``--create_db``)
---------------------------------------------

Creates the complete City2TABULA database infrastructure including CityDB schemas, application schemas, and imports all data.

**What this command does:**

- Creates CityDB infrastructure (citydb, citydb_pkg, lod2, lod3 schemas)
- Creates City2TABULA application schemas (city2tabula, tabula)
- Sets up all tables, functions, and procedures
- Imports supplementary data (TABULA building type data)
- Imports CityGML/CityJSON data into CityDB schemas

.. code-block:: bash

   # Complete database setup (run once)
   ./City2TABULA --create_db

Stage 2: Database Reset Operations
-----------------------------------

Different reset strategies for various scenarios:

**Complete Reset** (``--reset_all`` or ``--reset_db``):

Completely resets everything - drops all schemas and recreates the complete database.

- Drops ALL schemas (CityDB + City2TABULA)
- Recreates everything from scratch
- Imports all data
- **Use when**: You want to start completely fresh

.. code-block:: bash

   # Complete reset (everything)
   ./City2TABULA --reset_all

**CityDB-Only Reset** (``--reset_citydb``):

Resets only the CityDB infrastructure while preserving City2TABULA application data.

- Drops CityDB schemas (citydb, citydb_pkg, lod2, lod3)
- Recreates CityDB infrastructure
- Re-imports CityGML/CityJSON data
- **Preserves**: City2TABULA schemas and data
- **Use when**: CityDB data is corrupted, importing different CityGML files, or CityDB schema issues

.. code-block:: bash

   # Reset only CityDB (preserve application data)
   ./City2TABULA --reset_citydb

**City2TABULA-Only Reset** (``--reset_city2tabula``):

Resets only City2TABULA application schemas while preserving CityDB infrastructure.

- Drops City2TABULA schemas (city2tabula, tabula)
- Recreates application schemas and tables
- Re-imports supplementary data
- **Preserves**: CityDB infrastructure and data
- **Use when**: Application schema changes, TABULA data updates, or feature extraction pipeline issues

.. code-block:: bash

   # Reset only application schemas (preserve CityDB)
   ./City2TABULA --reset_city2tabula

Stage 3: Feature Extraction (``--extract_features``)
-----------------------------------------------------

Processes building data from CityDB schemas and extracts features for machine learning.

**Processing Flow:**

1. **Building Discovery**: Identifies all buildings in LOD2 and LOD3 schemas
2. **Batch Creation**: Organizes buildings into configurable batch sizes
3. **Parallel Processing**: Uses worker goroutines for concurrent processing
4. **Feature Pipeline**: Runs sequential jobs on each batch:

   - Extract child features (walls, roofs, windows)
   - Dump and analyze geometries
   - Calculate surface attributes
   - Compute building-level features
   - Calculate volumes and storeys
   - Detect attached neighbors
   - Label building features

.. code-block:: bash

   # Run feature extraction
   ./City2TABULA --extract_features

**Performance Characteristics:**

- Processes 64,400+ buildings per second
- 2.5-4x performance improvement with parallel architecture
- Configurable batch sizes and worker threads

Pipeline Architecture
---------------------

Batch Processing System
~~~~~~~~~~~~~~~~~~~~~~~

The feature extraction pipeline uses a sophisticated batch processing system:

1. **Building Discovery**: Queries CityDB schemas to find all building IDs
2. **Batch Creation**: Groups buildings into configurable batch sizes (default: 1000)
3. **Pipeline Queue**: Creates processing pipelines for each batch
4. **Worker Pool**: Distributes pipelines across concurrent worker goroutines

**Pipeline Jobs Sequence:**

Each batch processes through these sequential jobs:

1. ``01_get_child_feat.sql`` - Extract child features from buildings
2. ``02_dump_child_feat_geom.sql`` - Dump geometries for analysis
3. ``03_calc_child_feat_attr.sql`` - Calculate child feature attributes
4. ``04_calc_bld_feat.sql`` - Calculate building-level features
5. ``06_calc_volume.sql`` - Compute building volumes
6. ``07_calc_storeys.sql`` - Calculate number of storeys
7. ``08_calc_attached_neighbours.sql`` - Detect attached neighbors
8. ``09_label_building_features.sql`` - Apply feature labeling

Parallel Processing
~~~~~~~~~~~~~~~~~~~

- **Worker Goroutines**: Configurable number of parallel workers (default: 8)
- **Concurrent Batches**: Multiple batches processed simultaneously
- **Performance Scaling**: 2.5-4x improvement over sequential processing
- **Memory Efficiency**: Batch-based processing prevents memory exhaustion

Command Usage Patterns
-----------------------

Available Commands
~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # Show help and available options
   ./City2TABULA --help

   # Create complete database (CityDB + schemas + data)
   ./City2TABULA --create_db

   # Reset everything (complete fresh start)
   ./City2TABULA --reset_all

   # Reset only CityDB infrastructure (preserve application data)
   ./City2TABULA --reset_citydb

   # Reset only City2TABULA schemas (preserve CityDB)
   ./City2TABULA --reset_city2tabula

   # Extract features from existing data
   ./City2TABULA --extract_features

   # Legacy reset command (same as --reset_all)
   ./City2TABULA --reset_db

Common Workflows
~~~~~~~~~~~~~~~~

**Initial Setup Workflow:**

.. code-block:: bash

   # Complete setup in one command
   ./City2TABULA --create_db
   ./City2TABULA --extract_features

**Update CityGML Data Workflow:**

.. code-block:: bash

   # Place new CityGML files in data/lod2/ and data/lod3/
   ./City2TABULA --reset_citydb
   ./City2TABULA --extract_features

**Update TABULA Data Workflow:**

.. code-block:: bash

   # Update data/tabula/*.csv files
   ./City2TABULA --reset_city2tabula
   ./City2TABULA --extract_features

**Development Workflow (iterate on feature extraction):**

.. code-block:: bash

   # Preserve CityDB data while developing features
   ./City2TABULA --reset_city2tabula
   ./City2TABULA --extract_features

**Complete Reset Workflow:**

.. code-block:: bash

   # Fresh start (everything)
   ./City2TABULA --reset_all
   ./City2TABULA --extract_features

Schema Organization
~~~~~~~~~~~~~~~~~~~

The database is organized into clear functional areas:

.. code-block:: text

   Database: City2TABULA_<country>
   ├── CityDB Infrastructure
   │   ├── citydb (core schema)
   │   ├── citydb_pkg (functions)
   │   ├── lod2 (LOD2 CityGML data)
   │   └── lod3 (LOD3 CityGML data)
   └── City2TABULA Application
       ├── city2tabula (processing tables)
       └── tabula (reference data)

Development Best Practices
~~~~~~~~~~~~~~~~~~~~~~~~~~

**For CityGML Data Changes:**

.. code-block:: bash

   # 1. Update your CityGML files in data/lod2/ and data/lod3/
   # 2. Reset only CityDB (preserves application schemas)
   ./City2TABULA --reset_citydb
   # 3. Extract features
   ./City2TABULA --extract_features

**For Application Development:**

.. code-block:: bash

   # 1. Reset only application schemas (preserves CityDB data)
   ./City2TABULA --reset_city2tabula
   # 2. Extract features
   ./City2TABULA --extract_features

**For Complete Fresh Start:**

.. code-block:: bash

   # 1. Reset everything
   ./City2TABULA --reset_all
   # 2. Extract features
   ./City2TABULA --extract_features


Key Benefits
------------

Architecture Benefits
~~~~~~~~~~~~~~~~~~~~~

* **High-Performance Go Implementation**: Native concurrency and memory efficiency
* **Parallel Architecture**: Goroutine-based workers with configurable parallelism
* **Batch Processing**: Optimized batch sizes preventing memory exhaustion
* **Error Isolation**: Individual batch failures don't affect other processing
* **Scalable Design**: Handles 100K+ buildings efficiently

Processing Benefits
~~~~~~~~~~~~~~~~~~~

* **Multi-LOD Support**: Simultaneous LOD2 and LOD3 processing
* **High Throughput**: 64,400+ buildings per second capability
* **Memory Efficient**: Batch-based processing with configurable sizes
* **Query Optimization**: SQL template system with parameter injection
* **Connection Pooling**: Efficient database connection management

Development Benefits
~~~~~~~~~~~~~~~~~~~~

* **Granular Reset Options**: Reset only what you need (CityDB vs application schemas)
* **Clear Separation**: CityDB and application concerns are separated
* **Fast Iteration**: Preserve data while developing specific components
* **Better Error Recovery**: Issues in one area don't require complete rebuild
* **Self-Explanatory Commands**: Command names clearly indicate their purpose
* **Comprehensive Logging**: Detailed progress tracking and performance metrics
* **Configurable Parameters**: Adjustable batch sizes and worker counts
* **Fault Tolerance**: Graceful error handling and detailed error reporting

Workflow Benefits
~~~~~~~~~~~~~~~~~

* **Efficient Updates**: Import new CityGML data without rebuilding everything
* **Development Friendly**: Iterate on features without data re-import overhead
* **Production Ready**: Complete automation with single commands
* **Backward Compatible**: Legacy commands still work for existing scripts
* **Clear Documentation**: Each command's purpose and use case is documented

Performance Metrics
-------------------

The City2TABULA pipeline has been optimized for high-throughput processing:

**Processing Speed**
* 64,400+ buildings processed per second (actual performance varies by complexity)
* Parallel processing across multiple CPU cores
* Configurable worker count based on system resources

**Memory Management**
* Batch-based processing prevents memory exhaustion
* Configurable batch sizes (default: 5,000 buildings per batch)
* Connection pooling for database efficiency

**Monitoring**
* Real-time progress tracking with timestamps
* Performance metrics per processing stage
* Detailed error reporting with batch-level isolation

**Scalability Indicators**
* Successfully tested with datasets containing 100,000+ buildings
* Linear scaling with additional CPU cores
* Memory usage remains stable regardless of dataset size

.. note::
   Performance metrics are based on testing with German LOD2 datasets containing 100,000+ buildings.
   Actual performance may vary based on:
   
   * Hardware specifications (CPU cores, RAM, storage)
   * Database configuration and storage type
   * Network latency for remote databases
   * CityGML complexity and feature density

Output Schema
-------------

Database Schema Structure
~~~~~~~~~~~~~~~~~~~~~~~~~

The pipeline creates and populates the following schema structure:

**CityDB Schemas** (populated via external CityDB tools):

.. code-block:: sql

   -- LOD2 building data
   lod2.building
   lod2.thematic_surface
   lod2.building_installation

   -- LOD3 building data
   lod3.building
   lod3.thematic_surface
   lod3.building_installation

**Training Schema** (populated by feature extraction):

.. code-block:: sql

   -- LOD2 Feature Tables
   training.lod2_child_feature
   training.lod2_child_feature_geom_dump
   training.lod2_child_feature_surface
   training.lod2_building_feature

   -- LOD3 Feature Tables
   training.lod3_child_feature
   training.lod3_child_feature_geom_dump
   training.lod3_child_feature_surface
   training.lod3_building_feature

**Tabula Schema** (reference data):

.. code-block:: sql

   -- TABULA building type reference data
   tabula.building_types
   tabula.country_variants

Key Attributes Extracted
~~~~~~~~~~~~~~~~~~~~~~~~

* **Geometric Features**: Surface areas, volumes, heights, complexity metrics
* **Spatial Relationships**: Centroids, footprints, attached neighbor analysis
* **Structural Attributes**: Number of storeys, room heights, roof characteristics
* **Surface Analysis**: Wall/roof/window counts, areas, orientations, and tilt angles
* **Contextual Data**: Population density factors, postal code relationships
* **Classification Features**: ML-ready attributes for TABULA building type prediction

Data Processing Flow
~~~~~~~~~~~~~~~~~~~

1. **Raw CityDB Data** → Building geometries and metadata
2. **Child Feature Extraction** → Individual surface elements (walls, roofs, windows)
3. **Geometry Analysis** → Surface areas, orientations, and spatial relationships
4. **Building Aggregation** → Building-level summary statistics and features
5. **ML-Ready Output** → Labeled dataset suitable for Random Forest training

This pipeline transforms raw 3D building data into a comprehensive, machine learning-ready dataset for TABULA building type classification and urban analysis applications.