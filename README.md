![go](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml/badge.svg)

# City2TABULA

A high-performance Go-based tool for preparing 3D spatial data from CityDB and PostGIS-enabled PostgreSQL databases. The tool is part of a larger pipeline to classify OSM buildings into TABULA building types for heating demand estimation.

The pipeline processes spatial features such as attached neighbours, grid-based geometry relationships, and building characteristics from LOD2 and LOD3 3D building data. The extracted data is then used to train Random Forest (RF) models for automated building classification.

## Key Features

### **Core Processing Capabilities**
- **Building-Centric Parallel Processing**: Advanced parallel architecture processing 100K+ buildings efficiently
- **CityDB Integration**: Native support for 3D building data (LOD2/LOD3) in CityGML and CityJSON format from CityDB schemas
- **Parameterized SQL Templates**: Dynamic SQL scripts supporting multiple LOD levels with single templates
- **Batch Processing**: Optimized batch processing with configurable batch sizes for large datasets

### **Data Processing Pipeline**
- **Multi-LOD Support**: Process both LOD2 and LOD3 building data simultaneously
- **Spatial Analysis**: Building geometry analysis, volume calculations, and neighbour detection
- **Feature Extraction**: Child feature extraction (walls, roofs, windows) with geometric relationships
- **TABULA Integration**: Building type classification using TABULA methodology

### **Performance & Scalability**
- **Memory Efficient**: Batch-based processing preventing memory exhaustion
- **Parallel Architecture**: Goroutine-based workers achieving 2.5-4x performance improvements
- **Database Optimization**: Query plan caching and connection pooling

## System Requirements

### Software Dependencies

**Required:**
- **Go**: 1.21 or later (Download from [golang.org](https://go.dev/doc/install))

- **PostgreSQL**: 17+ with PostGIS 3.5+ (https://www.postgresql.org/download/)

- **PostGIS**: 3.5+ (https://postgis.net/install/)

- **Java**: 17+ for CityDB Tool (https://www.oracle.com/java/technologies/downloads/)

- **Git**: 2.25+ for source management (https://git-scm.com/downloads)

### CityDB Tool

- **CityDB Importer/Exporter**: v1.0.0 Download from [here](https://github.com/3dcitydb/citydb-tool/releases/tag/v1.0.0)

Unzip the downloaded file and place the `citydb-tool` directory in a known location (e.g., `/opt/citydb-tool` or `C:\Program Files\citydb-tool`).


## Pipeline Overview

The City2TABULA pipeline consists of several stages:

```mermaid
graph LR
    A[3D CityDB Setup] --> B[Data Import]
    B --> C[Feature Extraction]
    C --> D[Building Data with Features]
```


## Project Structure

> This tool is under active development. Therefore it may be subject to changes and improvements over time.

```
├── city2tabula                # Downloaded binary (renamed for convenience)
├── cmd
│   ├── main.go
│   └── test_file_grouping.go
├── data
│   ├── lod2
│   │   ├── czech
│   │   ├── germany
│   │   └── netherlands
│   ├── lod3
│   │   ├── austria
│   │   ├── czech
│   │   ├── germany
│   │   └── netherlands
│   ├── README.md
│   └── tabula
│       ├── austria.csv
│       ├── belgium.csv
│       ├── bulgaria.csv
│       ├── ...
│       ├── sweden.csv
│       └── united_kingdom.csv
├── docs
├── examples
├── format.
├── go.mod
├── go.sum
├── internal
│   ├── config
│   │   ├── batch.go
│   │   ├── citydb.go
│   │   ├── config.go
│   │   ├── database.go
│   │   ├── data.go
│   │   ├── env.go
│   │   └── sql.go
│   ├── db
│   │   ├── connection.go
│   │   └── setup.go
│   ├── importer
│   │   ├── citydb.go
│   │   └── supplementary.go
│   ├── process
│   │   ├── job.go
│   │   ├── orchestrator.go
│   │   ├── pipeline.go
│   │   ├── queue.go
│   │   ├── runner.go
│   │   └── worker.go
│   └── utils
│       ├── batch.go
│       ├── citydb.go
│       ├── exec.go
│       ├── logger.go
│       └── print.go
├── logs
│   ├── YYYY-MM-DD.log
├── README.md
└── sql
    ├── functions
    │   └── 01_surface_area_corrected_geom.sql
    ├── schema
    │   ├── 01_create_tabula_tables.sql
    │   └── 02_create_main_tables.sql
    └── scripts
        ├── main
        │   ├── 01_get_child_feat.sql
        │   ├── 02_dump_child_feat_geom.sql
        │   ├── 03_calc_child_feat_attr.sql
        │   ├── 04_calc_bld_feat.sql
        │   ├── 06_calc_volume.sql
        │   ├── 07_calc_storeys.sql
        │   ├── 08_calc_attached_neighbours.sql
        │   └── 09_label_building_features.sql
        └── supplementary
            └── 01_extract_tabula_attributes.sql
```

---
## Example Usage

### 1. Download executable and source code

All the release tags are available at the [Releases](https://github.com/THD-Spatial/City2TABULA/releases) page.

Via command line:
```bash
# Download the latest release
wget https://github.com/THD-Spatial/City2TABULA/archive/refs/tags/v0.2.0-alpha.zip

# Unzip and rename the directory
unzip v0.2.0-alpha.zip
mv City2TABULA-0.2.0-alpha City2TABULA
cd City2TABULA

# Download the appropriate binary for your system and rename for easy usage
# For Linux AMD64:
wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-linux-amd64
mv city2tabula-linux-amd64 city2tabula
chmod +x city2tabula

# For Linux ARM64:
# wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-linux-arm64
# mv city2tabula-linux-arm64 city2tabula
# chmod +x city2tabula

# For macOS Intel:
# wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-darwin-amd64
# mv city2tabula-darwin-amd64 city2tabula
# chmod +x city2tabula

# For macOS Apple Silicon:
# wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-darwin-arm64
# mv city2tabula-darwin-arm64 city2tabula
# chmod +x city2tabula

# For Windows AMD64:
# wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-windows-amd64.exe
# mv city2tabula-windows-amd64.exe city2tabula.exe

# For Windows ARM64:
# wget https://github.com/THD-Spatial/City2TABULA/releases/download/v0.2.0-alpha/city2tabula-windows-arm64.exe
# mv city2tabula-windows-arm64.exe city2tabula.exe

# Verify the binary works
./city2tabula --help
```

### 2. Prepare Data
Refer to the [data/README.md](data/README.md) for sample datasets and instructions on organizing your data.

### 3. Create Configuration File
```bash
# Copy example configuration
cp .env.example .env

# Edit configuration
nano .env
```
Update the `.env` file with your database credentials, CityDB tool path, and other settings as shown in [here](#configuration).

### 4. Run the Pipeline
```bash
# Create the database and import data
./city2tabula --create_db
```

### 5. Extract Features
```bash
./city2tabula --extract_features
```


## Setup Development Environment

### 1. Clone the Repository

```bash
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA
```

### 3. Checkout the feature branch (if applicable)

```bash
git checkout <feature-branch-name>
```

**Warning:** Do not use the main branch for development.

*If you are new to git, please refer to https://github.com/firstcontributions/first-contributions*

### 2. Configuration

**Create Environment File:**
```bash
# Copy example configuration
cp .env.example .env

# Edit configuration
nano .env
```

**Example `.env` Configuration:**
```bash
# Global Configuration
COUNTRY=germany # Specify the country you want to train from the list of available countries
# Available countries: austria, belgium, cyprus, czechia, denmark, france, germany,
# greece, hungary, ireland, italy, netherlands, norway, poland, serbia, slovenia,
# spain, sweden, united_kingdom

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=<your_pg_password_here>
DB_SSL_MODE=disable # enable it for production

# CityDB Configuration
CITYDB_TOOL_PATH=path/to/citydb-tool-1.X.X # Replace with actual path to CityDB tool
CITYDB_SRID=25832 # Coordinate Reference System for CityDB (UTM zone 32N for Germany)
CITYDB_SRS_NAME=ETRS89 / UTM zone 32N # Spatial Reference System Name for CityDB

# Parallel Processing Configuration
THREAD_COUNT=4        # Number of threads for parallel processing (optional)
DB_MAX_OPEN_CONNS=10   # Maximum number of open connections to the database
DB_MAX_IDLE_CONNS=5    # Maximum number of idle connections to the database

# Logging Configuration
LOG_LEVEL=INFO # Set the logging level (DEBUG, INFO, WARN, ERROR)
               # For development: DEBUG - shows all debug information
               # For production: INFO - shows essential information only
               # For monitoring: WARN - shows only warnings and errors
```

> **Note**: The available countries are based on TABULA and EPISCOPE project data. Each country has specific SRID configurations. Refer to `.env.example` for complete SRID mappings for all supported countries.


### 3. Initialise the Go Module (if not already)

```bash
go mod tidy
```

### 4. Build the Binary

```bash
go build -o city2tabula ./cmd
```

### 5. Verify Installation
```bash
# Test city2tabula (works for both downloaded binary and locally built)
./city2tabula --help
```

### 6. Prepare Data

- Download or obtain 3D city model data in CityGML or CityJSON format.
- Ensure data is organized in the following directory structure:

**Data Directory Structure:**
```
data/
├── lod2/
│   └── germany/
│       └── your-lod2-city.gml or your-lod2-city.json
├── lod3/
│   └── germany/
│       └── your-lod3-city.gml or your-lod3-city.json
└── tabula/
    └── germany.csv # Already included

# Note: You can add data for other countries by creating respective folders under lod2/ and lod3/ directories. Make sure to update the COUNTRY variable in the .env file accordingly.
```

### 7. Initialize Database
```bash
# Create complete database setup:
# - CityDB schemas (lod2, lod3)
# - Training and tabula schemas
# - Import supplementary data
./city2tabula --create_db
```


### 8. Extract Features
```bash
# Run feature extraction pipeline
./city2tabula --extract_features
```

---

### Available Commands

| Command | Description |
|---------|-------------|
| `--help` | Show help information |
| `--create_db` | Create the city2tabula database and CityDB schemas required to store the 3D city models and import the data |
| `--reset_db` | Reset the city2tabula database and CityDB schemas (drops all tables and re-creates them) |
| `--extract_features` | Run feature extraction pipeline |
| `--reset_city2tabula` | Reset only the city2tabula database (drops all tables and re-creates them). This option is useful when you want to make changes to SQL scripts for extracting features without affecting the entire database |

*For usage examples, refer to Database Commands documentation [here](/docs/source/DATABASE_COMMANDS.md).*

> **Tip:** If you are using vs code, you can use the provided [tasks.json](.vscode/tasks.json) to run the commands directly from the editor. Press Ctrl+Shift+P and type "Run Task" to see available tasks for running the commands listed in [Available Commands](#available-commands).

---

## Documentation

Comprehensive documentation is available at [City2TABULA ReadTheDocs](https://city2tabula.readthedocs.io/en/latest/)

- **Module Documentation** - Detailed API and architecture documentation
- **Configuration Reference** - All configuration options and tuning
- **Troubleshooting Guide** - Common issues and solutions

**Build Documentation Locally:**

```bash
pip install -r docs/requirements.txt
cd docs
sphinx-autobuild source build/html
```

## License

This project is licensed under the Apache License Version 2.0 - see the [LICENSE](/LICENSE) file for details.

> **Note**: This tool is under active development. Features and performance may evolve with future releases.

## Acknowledgments

This project is being developed in the context of the research project RENvolveIT (https://projekte.ffg.at/projekt/5127011).

This research was funded by CETPartnership, the Clean Energy Transition Partnership under the 2023 joint call for research proposals, co-funded by the European Commission (GA N°101069750) and with the funding organizations detailed on https://cetpartnership.eu/funding-agencies-and-call-mod-ules.​

 <img src="docs/source/img/CETP-logo.svg" alt="CETPartnership" width="144" height="72">  <img src="docs/source/img/EN_Co-fundedbytheEU_RGB_POS.png" alt="EU" width="180" height="40">

- **3DCityDB**: For providing the foundation for 3D spatial data management. (https://www.3dcitydb.org/)
