# City2TABULA Database Management Commands

This document explains the simplified database management commands for City2TABULA.

## Available Commands

### 1. `--create_db`
**Purpose**: Create the complete City2TABULA database from scratch
**What it does**:
- Creates CityDB infrastructure (citydb, citydb_pkg, lod2, lod3 schemas)
- Creates City2TABULA application schemas (city2tabula, tabula)
- Sets up all tables, functions, and indexes
- Imports all data (supplementary + CityGML/CityJSON)

**Use when**: Setting up the database for the first time

### 2. `--reset_all`
**Purpose**: Complete reset of everything
**What it does**:
- Drops ALL schemas (CityDB + City2TABULA)
- Recreates everything from scratch
- Imports all data

**Use when**: You want to start completely fresh

### 3. `--reset_citydb`
**Purpose**: Reset only the CityDB infrastructure
**What it does**:
- Drops CityDB schemas (citydb, citydb_pkg, lod2, lod3)
- Recreates CityDB infrastructure
- Re-imports CityGML/CityJSON data
- **Preserves**: City2TABULA schemas and data

**Use when**:
- CityDB data is corrupted
- You want to import different CityGML files
- CityDB schema issues need fixing

### 4. `--reset_city2tabula`
**Purpose**: Reset only City2TABULA application schemas
**What it does**:
- Drops City2TABULA schemas (city2tabula, tabula)
- Recreates application schemas and tables
- Re-imports supplementary data
- **Preserves**: CityDB infrastructure and data

**Use when**:
- Application schema changes
- TABULA data updates
- Feature extraction pipeline issues

### 5. `--extract_features`
**Purpose**: Run the feature extraction pipeline
**What it does**:
- Extracts features from existing CityDB data
- Runs SQL processing pipelines
- Generates training data

**Use when**: Ready to process features after database setup

### 6. `--reset_db` (Legacy)
**Purpose**: Backward compatibility
**What it does**: Same as `--reset_all`

## Command Usage Examples

```bash
# First time setup
./City2TABULA --create_db

# Complete reset (everything)
./City2TABULA --reset_all

# Reset only CityDB (keep application data)
./City2TABULA --reset_citydb

# Reset only application schemas (keep CityDB)
./City2TABULA --reset_city2tabula

# Run feature extraction
./City2TABULA --extract_features

# Check available options
./City2TABULA --help
```

## Common Workflows

### Workflow 1: Fresh Installation
```bash
./City2TABULA --create_db
./City2TABULA --extract_features
```

### Workflow 2: Update CityGML Data
```bash
# Place new CityGML files in data/lod2/ and data/lod3/
./City2TABULA --reset_citydb
./City2TABULA --extract_features
```

### Workflow 3: Update TABULA Data
```bash
# Update data/tabula/*.csv files
./City2TABULA --reset_city2tabula
./City2TABULA --extract_features
```

### Workflow 4: Complete Rebuild
```bash
./City2TABULA --reset_all
./City2TABULA --extract_features
```

## Schema Organization

```
Database: City2TABULA_<country>
├── CityDB Infrastructure
│   ├── citydb (core schema)
│   ├── citydb_pkg (functions)
│   ├── lod2 (LOD2 CityGML data)
│   └── lod3 (LOD3 CityGML data)
└── City2TABULA Application
    ├── city2tabula (processing tables)
    └── tabula (reference data)
```

## Benefits of the New Structure

1. **Clear Separation**: CityDB and application concerns are separated
2. **Granular Control**: Reset only what you need
3. **Better Error Recovery**: Issues in one part don't require full rebuild
4. **Efficient Updates**: Update data without rebuilding everything
5. **Self-Explanatory**: Command names clearly indicate what they do