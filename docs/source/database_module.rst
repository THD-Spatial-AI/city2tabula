Database Module Documentation
=============================

The ``internal/db`` package provides database connectivity, schema management, and setup operations for the City2TABULA pipeline.

Overview
--------

The database module handles:

- **Connection Management**: PostgreSQL connection pooling with pgx/v5
- **Schema Setup**: Automated creation of CityDB and training schemas
- **Extension Management**: PostGIS extension configuration
- **Migration Support**: Database reset and upgrade operations

Package Structure
-----------------

.. code-block:: text

   internal/db/
   ├── connection.go    # Database connection and pooling
   └── setup.go        # Schema creation and management

Connection Management
---------------------

Database Connectivity
~~~~~~~~~~~~~~~~~~~~~~

The connection module provides robust database connectivity using connection pooling:

.. code-block:: go

   // Connect to database with connection pooling
   pool, err := db.ConnectPool(config)
   if err != nil {
       log.Fatalf("Failed to connect: %v", err)
   }
   defer pool.Close()

**Key Features:**

- **Connection Pooling**: Efficient management of database connections
- **Health Checks**: Automatic connection validation and recovery
- **SSL Support**: Configurable SSL/TLS encryption
- **Timeout Handling**: Connection and query timeout management

**Configuration Options:**

.. code-block:: go

   type DBConfig struct {
       Host     string  // Database hostname
       Port     string  // Database port (default: 5432)
       Name     string  // Database name
       User     string  // Database username
       Password string  // Database password
       SSLMode  string  // SSL mode (disable, require, verify-full)
   }

Connection Pool Settings
~~~~~~~~~~~~~~~~~~~~~~~~

**Default Pool Configuration:**

- **Max Connections**: 25 connections
- **Min Connections**: 5 connections
- **Connection Lifetime**: 1 hour
- **Connection Timeout**: 30 seconds

**Tuning for Performance:**

.. code-block:: go

   // Recommended production settings
   poolConfig.MaxConns = int32(config.Batch.Threads * 2)
   poolConfig.MinConns = int32(config.Batch.Threads)
   poolConfig.MaxConnLifetime = time.Hour
   poolConfig.MaxConnIdleTime = time.Minute * 30

Schema Management
-----------------

CityDB Schema Setup
~~~~~~~~~~~~~~~~~~~

The setup module automates the creation of 3D CityDB schemas for LOD2 and LOD3 data:

**Core Functions:**

.. code-block:: go

   // Create CityDB schemas
   func CreateCityDB(config *Config) error

   // Reset CityDB schemas
   func ResetCityDB(config *Config, pool *pgxpool.Pool) error

**Schema Creation Process:**

1. **Execute CityDB Tool**: Runs citydb-tool to create schemas
2. **Configure Spatial Reference**: Sets up coordinate reference system
3. **Apply Extensions**: Enables required PostGIS extensions
4. **Validate Setup**: Confirms schema creation success

**Generated Schemas:**

- **citydb**: Core CityDB schema with base tables
- **citydb_pkg**: CityDB packages and functions
- **lod2**: Level of Detail 2 building data
- **lod3**: Level of Detail 3 building data

Training Schema Setup
~~~~~~~~~~~~~~~~~~~~~

Creates schemas for feature extraction and machine learning training:

**Core Functions:**

.. code-block:: go

   // Create training schemas
   func Create3D2TabulaDB(config *Config, pool *pgxpool.Pool) error

   // Reset training schemas
   func Reset3DToTabulaDB(config *Config, pool *pgxpool.Pool) error

**Generated Schemas:**

- **training**: Feature extraction results and intermediate data
- **tabula**: TABULA building type classifications and reference data

Schema Structure
~~~~~~~~~~~~~~~~

**Training Schema Tables:**

+----------------------------------------+------------------------------------------+
| Table                                  | Purpose                                  |
+========================================+==========================================+
| ``lod2_child_feature_geom_dump``       | Building component geometries (LOD2)    |
+----------------------------------------+------------------------------------------+
| ``lod2_child_feature_surface``         | Surface analysis results (LOD2)         |
+----------------------------------------+------------------------------------------+
| ``lod2_building_feature``              | Aggregated building features (LOD2)     |
+----------------------------------------+------------------------------------------+
| ``lod3_child_feature_geom_dump``       | Building component geometries (LOD3)    |
+----------------------------------------+------------------------------------------+
| ``lod3_child_feature_surface``         | Surface analysis results (LOD3)         |
+----------------------------------------+------------------------------------------+
| ``lod3_building_feature``              | Aggregated building features (LOD3)     |
+----------------------------------------+------------------------------------------+
| ``training_data``                      | Final labeled dataset for ML            |
+----------------------------------------+------------------------------------------+

**Tabula Schema Tables:**

+----------------------------------------+------------------------------------------+
| Table                                  | Purpose                                  |
+========================================+==========================================+
| ``tabula``                             | TABULA building type classifications    |
+----------------------------------------+------------------------------------------+
| ``tabula_variant``                     | Building type variants and subtypes     |
+----------------------------------------+------------------------------------------+

PostGIS Extension Management
----------------------------

Extension Setup
~~~~~~~~~~~~~~~

The database module automatically configures required PostGIS extensions:

**Core Extensions:**

.. code-block:: sql

   -- Primary PostGIS extension
   CREATE EXTENSION IF NOT EXISTS postgis;

   -- SFCGAL for advanced 3D operations
   CREATE EXTENSION IF NOT EXISTS postgis_sfcgal;

   -- Raster support (PostGIS 3.5 and earlier)
   CREATE EXTENSION IF NOT EXISTS postgis_raster;

**Version Compatibility:**

- **PostGIS 3.6+**: Integrated raster support, no separate raster extension needed
- **SFCGAL**: Graceful fallback when not available
- **Version Detection**: Automatic version detection and compatibility handling

Error Handling and Recovery
---------------------------

Connection Error Handling
~~~~~~~~~~~~~~~~~~~~~~~~~~

**Automatic Recovery:**

- Connection pool health monitoring
- Automatic reconnection on connection loss
- Graceful degradation during database maintenance

**Error Types:**

.. code-block:: go

   // Connection errors
   var (
       ErrConnectionFailed = errors.New("database connection failed")
       ErrPoolExhausted   = errors.New("connection pool exhausted")
       ErrTimeout         = errors.New("connection timeout")
   )

Schema Setup Error Handling
~~~~~~~~~~~~~~~~~~~~~~~~~~~~

**Common Issues and Solutions:**

1. **Missing CityDB Tool**:

   .. code-block:: text

      Error: citydb-tool not found in PATH
      Solution: Install CityDB tool and add to PATH

2. **Permission Issues**:

   .. code-block:: text

      Error: permission denied for schema creation
      Solution: Grant CREATE privileges to database user

3. **Extension Missing**:

   .. code-block:: text

      Error: extension "postgis" is not available
      Solution: Install PostGIS packages for your PostgreSQL version

Database Operations
-------------------

Query Execution
~~~~~~~~~~~~~~~~

**Prepared Statements:**

.. code-block:: go

   // Execute parameterized query
   func ExecuteQuery(pool *pgxpool.Pool, query string, args ...interface{}) error

   // Execute query with result scanning
   func QueryRows(pool *pgxpool.Pool, query string, args ...interface{}) (pgx.Rows, error)

**Transaction Management:**

.. code-block:: go

   // Execute multiple operations in transaction
   func ExecuteTransaction(pool *pgxpool.Pool, operations []Operation) error

**Batch Operations:**

.. code-block:: go

   // Execute batch of similar operations
   func ExecuteBatch(pool *pgxpool.Pool, batch *pgx.Batch) error

Performance Optimization
-------------------------

Connection Pool Tuning
~~~~~~~~~~~~~~~~~~~~~~~

**Memory Usage Optimization:**

.. code-block:: go

   // Calculate optimal pool size based on available memory
   func CalculateOptimalPoolSize(availableRAM int64, expectedConcurrency int) int32 {
       // Rule of thumb: 2-3 connections per worker thread
       baseConnections := expectedConcurrency * 2

       // Memory constraint: ~10MB per connection
       memoryConstraint := int(availableRAM / (10 * 1024 * 1024))

       return min(baseConnections, memoryConstraint)
   }

**Query Performance:**

- **Connection Reuse**: Minimize connection establishment overhead
- **Prepared Statements**: Cache query plans for repeated operations
- **Batch Processing**: Group operations to reduce round trips

Database Maintenance
--------------------

Vacuum and Analyze
~~~~~~~~~~~~~~~~~~

**Automated Maintenance:**

.. code-block:: sql

   -- After large data imports
   VACUUM ANALYZE training.lod2_child_feature_surface;
   VACUUM ANALYZE training.lod2_building_feature;

   -- Reclaim space after deletions
   VACUUM FULL training.training_data;

**Index Maintenance:**

.. code-block:: sql

   -- Rebuild spatial indexes for optimal performance
   REINDEX INDEX training.lod2_child_feature_surface_geom_idx;

Monitoring and Diagnostics
---------------------------

Connection Monitoring
~~~~~~~~~~~~~~~~~~~~~

**Active Connection Tracking:**

.. code-block:: go

   // Monitor connection pool status
   func MonitorConnectionPool(pool *pgxpool.Pool) {
       stats := pool.Stat()
       log.Printf("Active connections: %d/%d",
                 stats.AcquiredConns(), stats.MaxConns())
       log.Printf("Idle connections: %d", stats.IdleConns())
   }

**Performance Metrics:**

- Connection acquisition time
- Query execution duration
- Connection pool utilization
- Failed connection attempts

Database Size Monitoring
~~~~~~~~~~~~~~~~~~~~~~~~

**Storage Usage Tracking:**

.. code-block:: sql

   -- Monitor database size growth
   SELECT
       schemaname,
       tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
   FROM pg_tables
   WHERE schemaname IN ('training', 'tabula', 'lod2', 'lod3')
   ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

Usage Examples
--------------

Basic Database Setup
~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   package main

   import (
       "City2TABULA/internal/config"
       "City2TABULA/internal/db"
   )

   func main() {
       // Load configuration
       config := config.LoadConfig()

       // Create database connection
       pool, err := db.ConnectPool(config)
       if err != nil {
           log.Fatalf("Connection failed: %v", err)
       }
       defer pool.Close()

       // Set up CityDB schemas
       if err := db.CreateCityDB(config); err != nil {
           log.Fatalf("CityDB setup failed: %v", err)
       }

       // Set up training schemas
       if err := db.Create3D2TabulaDB(config, pool); err != nil {
           log.Fatalf("Training DB setup failed: %v", err)
       }
   }

Advanced Connection Management
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: go

   // Custom connection pool configuration
   func setupCustomPool(config *config.Config) (*pgxpool.Pool, error) {
       poolConfig, err := pgxpool.ParseConfig(config.DB.ConnectionString())
       if err != nil {
           return nil, err
       }

       // Customize pool settings
       poolConfig.MaxConns = 50
       poolConfig.MinConns = 10
       poolConfig.MaxConnLifetime = time.Hour * 2

       return pgxpool.ConnectConfig(context.Background(), poolConfig)
   }

For more information on configuration, see :doc:`config_module`.