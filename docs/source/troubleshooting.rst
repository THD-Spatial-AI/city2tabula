Troubleshooting Guide
====================

This guide helps you diagnose and resolve common issues encountered when using City2TABULA.

Common Issues and Solutions
---------------------------

Database Connection Issues
~~~~~~~~~~~~~~~~~~~~~~~~~~

**Issue: Database Connection Failed**

.. code-block:: text

   ERROR: Failed to connect to database: connection refused

**Possible Causes and Solutions:**

1. **PostgreSQL not running**:

   .. code-block:: bash

      # Check PostgreSQL status
      sudo systemctl status postgresql

      # Start PostgreSQL if stopped
      sudo systemctl start postgresql

2. **Incorrect connection parameters**:

   .. code-block:: bash

      # Verify .env configuration
      DB_HOST=localhost
      DB_PORT=5432
      DB_USER=your_username
      DB_PASSWORD=your_password

3. **Firewall blocking connection**:

   .. code-block:: bash

      # Check if port 5432 is open
      sudo ufw status
      sudo ufw allow 5432

**Issue: Permission Denied for User**

.. code-block:: text

   ERROR: permission denied for schema creation

**Solution:**

.. code-block:: sql

   -- Grant necessary privileges
   ALTER USER your_username CREATEDB;
   GRANT CREATE ON SCHEMA public TO your_username;

PostGIS Extension Issues
~~~~~~~~~~~~~~~~~~~~~~~~

**Issue: PostGIS Extension Not Available**

.. code-block:: text

   ERROR: extension "postgis" is not available

**Solutions:**

1. **Install PostGIS packages**:

   .. code-block:: bash

      # Ubuntu/Debian
      sudo apt install postgresql-postgis

      # CentOS/RHEL
      sudo dnf install postgis

2. **Enable PostGIS extension**:

   .. code-block:: sql

      -- Connect to your database
      \c City2TABULA_germany
      CREATE EXTENSION postgis;

**Issue: PostGIS Version Compatibility**

.. code-block:: text

   WARN: PostGIS Raster extension not available (likely PostGIS 3.6+)

**Explanation:**
This is expected behavior. PostGIS 3.6+ integrates raster functionality without requiring a separate extension. City2TABULA handles this gracefully.

**Issue: SFCGAL Functions Not Available**

.. code-block:: text

   ERROR: function cg_isplanar(geometry) does not exist

**Solution:**
City2TABULA includes fallback implementations for systems without SFCGAL. The error should be automatically resolved by using alternative planarity checks.

CityDB Tool Issues
~~~~~~~~~~~~~~~~~~

**Issue: CityDB Tool Not Found**

.. code-block:: text

   ERROR: citydb-tool not found in PATH

**Solutions:**

1. **Install CityDB Tool**:

   .. code-block:: bash

      # Download and install CityDB Tool
      wget https://github.com/3dcitydb/citydb-tool/releases/download/v1.0.0/citydb-tool-1.0.0.zip
      unzip citydb-tool-1.0.0.zip
      sudo mv citydb-tool-1.0.0 /opt/citydb-tool

      # Add to PATH
      echo 'export PATH=$PATH:/opt/citydb-tool' >> ~/.bashrc
      source ~/.bashrc

2. **Verify installation**:

   .. code-block:: bash

      citydb-tool --version

Data Import Issues
~~~~~~~~~~~~~~~~~~

**Issue: Invalid CityGML Files**

.. code-block:: text

   ERROR: XML parsing failed

**Solutions:**

1. **Validate CityGML files**:

   .. code-block:: bash

      # Check file integrity
      xmllint --noout your-file.gml

2. **Check file encoding**:

   .. code-block:: bash

      # Verify UTF-8 encoding
      file -i your-file.gml

**Issue: Duplicate Key Violations**

.. code-block:: text

   ERROR: duplicate key value violates unique constraint

**Solutions:**

1. **Reset database**:

   .. code-block:: bash

      City2TABULA --reset_db


Performance Issues
~~~~~~~~~~~~~~~~~~

**Issue: Slow Processing Performance**

1. **Review configuration**:

   .. code-block:: bash

      # Optimize for your system
      BATCH_SIZE=1000      # Reduce if memory limited
      BATCH_THREADS=8      # Match CPU cores

**Issue: Memory Exhaustion**

.. code-block:: text

   ERROR: out of memory

**Solutions:**

1. **Reduce batch size**:

   .. code-block:: bash

      # In .env file
      BATCH_SIZE=500       # Reduce from default 1000

**Issue: Database Query Timeouts**

.. code-block:: text

   ERROR: query timeout exceeded

**Solutions:**

1. **Optimize PostgreSQL settings**:

   .. code-block:: sql

      -- Increase query timeout
      ALTER SYSTEM SET statement_timeout = '60s';
      SELECT pg_reload_conf();

2. **Add database indexes**:

   .. code-block:: sql

      -- Create spatial indexes if missing
      CREATE INDEX IF NOT EXISTS building_geom_idx
      ON lod2.building USING gist(lod2_brep_geometry);

SQL Script Issues
~~~~~~~~~~~~~~~~~

**Issue: SQL Syntax Errors**

.. code-block:: text

   ERROR: syntax error at or near

**Solutions:**

1. **Check SQL template parameters**:
   Ensure all template variables (e.g., `{building_ids}`, `{city2tabula_schema}`) are properly replaced.

2. **Validate SQL manually**:

   .. code-block:: bash

      # Test SQL script manually
      psql -h localhost -U your_username -d City2TABULA_germany -f sql/scripts/main/script.sql

**Issue: Missing Schema References**

.. code-block:: text

   ERROR: schema "training" does not exist

**Solution:**

.. code-block:: bash

   # Ensure database setup completed
   City2TABULA --create_db

Configuration Issues
~~~~~~~~~~~~~~~~~~~~

**Issue: Invalid Configuration Values**

.. code-block:: text

   ERROR: Invalid configuration

**Solutions:**

1. **Validate .env file format**:

   .. code-block:: bash

      # Check for syntax errors
      source .env
      echo "Config loaded successfully"

2. **Use configuration validation**:

   .. code-block:: bash

      # City2TABULA validates config on startup
      City2TABULA --help  # This loads and validates config

**Issue: Missing Required Environment Variables**

.. code-block:: text

   ERROR: required environment variable not set

**Solution:**

.. code-block:: bash

   # Ensure all required variables are set
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=your_username
   DB_PASSWORD=your_password
   COUNTRY=germany


Performance Profiling
~~~~~~~~~~~~~~~~~~~~~~

**CPU Profiling:**

.. code-block:: bash

   # Profile CPU usage
   go tool pprof http://localhost:6060/debug/pprof/profile

**Database Connection Monitoring:**

.. code-block:: sql

   -- Monitor connection usage
   SELECT count(*) as active_connections,
          backend_type
   FROM pg_stat_activity
   GROUP BY backend_type;

Recovery Procedures
-------------------

Database Recovery
~~~~~~~~~~~~~~~~~

**Complete Database Reset:**

.. code-block:: bash

   # Full reset and rebuild
   City2TABULA --reset_db

**Partial Recovery:**

.. code-block:: bash

   # Reset only training data
   City2TABULA --reset_3d_to_tabula_db

Configuration Recovery
~~~~~~~~~~~~~~~~~~~~~~

**Reset to Default Configuration:**

.. code-block:: bash

   # Backup current config
   cp .env .env.backup

   # Copy default configuration
   cp .env.example .env

   # Edit with your specific values
   nano .env