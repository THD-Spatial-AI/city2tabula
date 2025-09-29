Quick Start Guide
=================

This guide will help you get City2TABULA up and running quickly with a sample dataset.

Prerequisites
-------------

Before starting, ensure you have completed the :doc:`installation` process and have:

- **Go**: 1.21 or later
- **PostgreSQL**: 17+ with PostGIS 3.5+
- **PostGIS**: 3.5+
- **Java**: 17+ for CityDB Tool
- **Git**: 2.25+ for source management
- **CityDB Importer/Exporter**: v1.0.0
- City2TABULA binary available
- Environment configuration file (`.env`) set up

Quick Setup
-----------

1. Verify Your Installation
~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # Check City2TABULA installation
   ./City2TABULA --help


2. Prepare Sample Data
~~~~~~~~~~~~~~~~~~~~~~

**Download Sample Building Data:**

For this quick start, you'll need LOD2/LOD3 building data in CityGML format. You can:

- Use your own CityGML files
- Download sample data from official CityDB repositories
- Use the provided example files in the `data/` directory

**Data Directory Structure:**

.. code-block:: text

   data/
   ├── lod2/
   │   └── germany/
   │       └── your-lod2-city.gml or your-lod2-city.json
   ├── lod3/
   │   └── germany/
   │       └── your-lod3-city.gml or your-lod3-city.json
   └── tabula/
       └── germany.csv # Already included

3. Initialize the Database
~~~~~~~~~~~~~~~~~~~~~~~~~~

**Create the Complete Database Setup:**

.. code-block:: bash

   # This command will:
   # 1. Create CityDB schemas (lod2, lod3)
   # 2. Create training and tabula schemas
   # 3. Import supplementary data
   ./City2TABULA --create_db

**Expected Output:**

.. code-block:: text

   INFO: Database connection established
   INFO: CityDB schema lod2 created successfully
   INFO: CityDB schema lod3 created successfully
   INFO: Training database setup completed
   INFO: Tabula data imported successfully
   INFO: Total runtime: 2.1s

4. Extract Features
~~~~~~~~~~~~~~~~~~~

**Run Feature Extraction Pipeline:**

.. code-block:: bash

   # Extract building features for machine learning
   ./City2TABULA --extract_features

**Pipeline Progress:**

.. code-block:: text

   INFO: Found 1500 buildings for lod2 in CityDB
   INFO: Found 800 buildings for lod3 in CityDB
   INFO: Created 15 batches for LOD2
   INFO: Created 8 batches for LOD3
   INFO: Starting 8 workers for parallel processing
   INFO: Processing batch 1/15...
   INFO: Feature extraction completed successfully
   INFO: Total runtime: 45.2s

Common Commands
---------------

Available Commands
~~~~~~~~~~~~~~~~~~

.. code-block:: bash

   # Show help information
   ./City2TABULA --help

   # Create the City2TABULA database and CityDB schemas
   ./City2TABULA --create_db

   # Reset the entire database and CityDB schemas
   ./City2TABULA --reset_db

   # Reset only the City2TABULA database
   ./City2TABULA --reset_City2TABULA

   # Run feature extraction pipeline
   ./City2TABULA --extract_features

Understanding the Output
------------------------

Feature Extraction Results
~~~~~~~~~~~~~~~~~~~~~~~~~~

After running feature extraction, your database will contain:

**Training Schema Tables:**

- `lod2_child_feature_geom_dump`: Geometries of building components
- `lod2_child_feature_surface`: Surface analysis (area, tilt, azimuth)
- `lod2_building_feature`: Aggregated building-level features
- `training_data`: Final labeled dataset for ML training

**Key Metrics Generated:**

- Surface areas (walls, roofs, windows)
- Building orientation and tilt angles
- Volume and storey calculations
- Neighbor relationships
- Population density factors

Data Quality Checks
~~~~~~~~~~~~~~~~~~~

**Verify Data Import:**

.. code-block:: bash

   # Check building count
   psql -h localhost -U your_username -d City2TABULA_germany -c "
   SELECT
     'lod2' as lod_level, COUNT(*) as building_count
   FROM lod2.building
   UNION ALL
   SELECT
     'lod3' as lod_level, COUNT(*) as building_count
   FROM lod3.building;"

**Check Feature Extraction Results:**

.. code-block:: bash

   # Verify extracted features
   psql -h localhost -U your_username -d City2TABULA_germany -c "
   SELECT
     objectclass_id,
     COUNT(*) as surface_count,
     AVG(surface_area) as avg_surface_area
   FROM training.lod2_child_feature_surface
   GROUP BY objectclass_id;"

Performance Optimization
------------------------

Configuration Setup
~~~~~~~~~~~~~~~~~~~

**Create Environment File:**

.. code-block:: bash

   # Copy example configuration
   cp .env.example .env

   # Edit configuration
   nano .env

**Example `.env` Configuration:**

.. code-block:: bash

   # Database Configuration
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=your_username
   DB_PASSWORD=your_password
   DB_SSL_MODE=disable

   # Country/Region (affects database name: City2TABULA_{country})
   COUNTRY=germany

   # CityDB Tool Path
   CITYDB_TOOL_PATH=/opt/citydb-tool

   # Processing Configuration
   BATCH_SIZE=1000        # Buildings per batch
   BATCH_THREADS=8        # Parallel worker threads

   # Logging
   LOG_LEVEL=INFO

Batch Configuration
~~~~~~~~~~~~~~~~~~~

Adjust batch processing settings in your `.env` file:

.. code-block:: bash

   # For smaller datasets or limited memory
   BATCH_SIZE=500
   BATCH_THREADS=4

   # For large datasets with ample resources
   BATCH_SIZE=2000
   BATCH_THREADS=16

Monitoring Progress
~~~~~~~~~~~~~~~~~~~

**Real-time Log Monitoring:**

.. code-block:: bash

   # Monitor current day's logs
   tail -f logs/$(date +%Y-%m-%d).log

**Performance Metrics:**

The pipeline provides detailed performance metrics:

- Buildings processed per second (64,400+ buildings/second capability)
- Batch processing times
- Memory usage patterns
- Database query performance
- 2.5-4x performance improvements with parallel architecture

Next Steps
----------

Now that you have City2TABULA running:

1. **Explore the Data**: Use PostgreSQL/PostGIS tools to examine the extracted features
2. **Machine Learning**: Use the generated training data with Python ML frameworks
3. **Scale Up**: Process larger datasets with optimized batch configurations
4. **Integration**: Integrate City2TABULA into your automated workflows

**Recommended Reading:**

- :doc:`config_module` - Understand configuration options
- :doc:`pipeline_workflow` - Deep dive into the processing pipeline
- :doc:`performance_optimization` - Optimize for your use case
- :doc:`troubleshooting` - Solve common issues

Troubleshooting
---------------

**Common Issues:**

1. **Memory Issues**: Reduce `BATCH_SIZE` in configuration
2. **Slow Performance**: Increase `BATCH_THREADS` if you have sufficient CPU cores
3. **Database Errors**: Check PostgreSQL logs and connection settings
4. **Missing Data**: Verify CityGML file paths and formats

For detailed troubleshooting, see :doc:`troubleshooting`.