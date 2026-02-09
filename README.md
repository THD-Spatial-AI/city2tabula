[![go](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml/badge.svg)](https://github.com/THD-Spatial/City2TABULA/actions/workflows/go.yml)
&nbsp;
[![rtd](https://app.readthedocs.org/projects/city2tabula/badge/?version__slug=latest)](https://city2tabula.readthedocs.io/en/latest/)
&nbsp;
[![GitHub release](https://img.shields.io/github/v/release/THD-Spatial/City2TABULA.svg)](https://github.com/THD-Spatial/City2TABULA/releases)

# City2TABULA

City2TABULA is a high-performance, Go-based data preparation tool for 3D building datasets stored in PostgreSQL/PostGIS using CityDB schemas. Its primary purpose is to extract, normalise, and enrich geometric and spatial attributes from LoD2 and LoD3 building models, enabling downstream tasks such as building typology classification and heating demand estimation.

The tool is designed as an upstream component within a larger research and modelling pipeline, where prepared building-level features are later consumed by machine learning models or energy calculation services (for example, TABULA-based workflows).

City2TABULA focuses on scalable, database-centric processing of large national or city-scale datasets, avoiding country-specific assumptions and minimising manual intervention.

---

## Quick Start with Docker (Recommended)

The fastest way to get started with City2TABULA is using Docker. This approach automatically handles all dependencies and setup.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) 20.10+
- [Docker Compose](https://docs.docker.com/compose/install/) 2.0+

**For Linux/macOS users:**

- `make` (usually pre-installed)

**For Windows users:**

- Windows batch scripts are provided (`setup.bat`)
- Or PowerShell scripts (`setup.ps1`)
- No need to install `make`

### Interactive Setup

**Linux/macOS:**

```bash
# Clone the repository
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA

# Interactive setup with Docker
make setup
```

**Windows (Command Prompt):**

```cmd
# Clone the repository
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA

# Interactive setup with Docker
setup.bat setup
```

**Windows (PowerShell):**

```powershell
# Clone the repository
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA

# Interactive setup with Docker
.\setup.ps1 setup
```

The setup command will:

1. Build the Docker environment
2. Show available countries with SRID information
3. Prompt you to select your target country
4. Ask for your PostgreSQL password (hidden input)
5. Configure everything automatically based on your selections
6. Start the containers

The Docker environment includes:

- Go 1.23.3 runtime (automatically configured)
- Java 25 with CityDB Tool 1.1.0 (automatically downloaded)
- All system dependencies (PostGIS tools, GDAL, etc.)
- Sample data and configuration
- Development environment ready to use

### Docker Commands

Run the following command to see all available Docker commands:

```bash
# Linux/macOS
make help

# Windows (Command Prompt)
setup.bat help
# or (PowerShell)
.\setup.ps1 help
```

### Complete Docker Workflow

**Step 1: Complete interactive setup**

```bash
# Linux/macOS
make setup

# Windows
setup.bat setup
# or
.\setup.ps1 setup
```

**Step 2: Access development shell**

```bash
# Linux/macOS
make dev

# Windows
setup.bat dev
# or
.\setup.ps1 dev
```

**Step 3: Inside the container, run the pipeline**

```bash
# Setup database
./city2tabula -create_db

# Extract features
./city2tabula -extract_features

# Show all options
./city2tabula -help
```

### Data Management with Docker

Place your 3D data files in the `data` directory before starting:

```bash
data/
├── lod2/germany/your-city.gml
├── lod3/germany/your-city.gml
└── tabula/germany.csv (included)
```

The data directory is automatically mounted into the container.

### Docker Configuration

The Docker environment provides intelligent configuration management through an interactive setup wizard that:

- Shows all 19 supported countries with their SRID and SRS information
- Automatically sets the correct SRID and SRS name based on your country selection

Example configuration result:

```bash
# Automatically set based on your selections
COUNTRY=netherlands
CITYDB_SRID=28992
CITYDB_SRS_NAME=Amersfoort / RD New
DB_PASSWORD=your_actual_password

# Other settings (automatically configured)
DB_HOST=172.17.0.1
DB_PORT=5432
DB_USER=postgres
CITYDB_TOOL_PATH=/usr/local/citydb-tool/citydb-tool-1.1.0/bin/citydb
```

## Manual Installation

If you prefer to install dependencies manually or need a custom setup for development purpose, follow these steps:

> [!NOTE]
> Current version of City2TABULA only supports Linux environments. Windows users are encouraged to use the Docker setup for a seamless experience.

### Software Dependencies

**Required:**

- **Go**: 1.21 or later (Download from [golang.org](https://go.dev/doc/install))
- **PostgreSQL**: 17+ with PostGIS 3.5+ (Download from [postgresql.org](https://www.postgresql.org/download/))
- **PostGIS**: 3.5+ (Download from [postgis.net](https://postgis.net/install/))
- **Java**: 17+ for CityDB Tool (Download from [oracle.com](https://www.oracle.com/java/technologies/downloads/))
- **Git**: 2.25+ for source management (Download from [git-scm.com](https://git-scm.com/downloads))
- **CityDB Importer/Exporter**: v1.0.0 (Download from [github.com](https://github.com/3dcitydb/citydb-tool/releases/tag/v1.0.0))

  - Unzip the downloaded file and place the `citydb-tool` directory in a known location (e.g., `/opt/citydb-tool` or `C:\Program Files\citydb-tool`).

### Installation Steps

#### 1. Clone the Repository

```bash
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA
```

#### 2. Create Configuration File

```bash
# Copy example configuration
cp .env.example .env

# Edit configuration
nano .env
```

**Example .env Configuration:**

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

# Logging Configuration
LOG_LEVEL=INFO # Set the logging level (DEBUG, INFO, WARN, ERROR)
               # For development: DEBUG - shows all debug information
               # For production: INFO - shows essential information only
               # For monitoring: WARN - shows only warnings and errors
```

#### 3. Initialize Go Module

```bash
go mod tidy
```

#### 4. Build the Binary

```bash
go build -o city2tabula ./cmd
```

#### 5. Verify Installation

```bash
./city2tabula -help
```

#### 6. Prepare Data

Download or obtain 3D city model data in CityGML or CityJSON format and organize in this structure:

```bash
data/
├── lod2/
│   └── germany/
│       └── your-lod2-city.gml or your-lod2-city.json
├── lod3/
│   └── germany/
│       └── your-lod3-city.gml or your-lod3-city.json
└── tabula/
    └── germany.csv # Already included
```

#### 7. Initialize Database

```bash
./city2tabula -create_db
```

Creates complete database setup:

- CityDB schemas (lod2, lod3)
- Training and tabula schemas
- Import supplementary data

#### 8. Extract Features

```bash
# Run feature extraction pipeline
./city2tabula -extract_features
```

### Alternative: Binary Download Method

For users who prefer downloading pre-built binaries:

1. Download Source Code and Binary from [Release](https://github.com/THD-Spatial/City2TABULA/releases)

2. Make Executable and Verify

```bash
# Unix-like systems
mv city2tabula-<your_platform> city2tabula
# e.g., mv city2tabula-linux-amd64 city2tabula

chmod +x city2tabula
./city2tabula -help

# Windows
.\city2tabula.exe -help
```

3. Follow steps 2-8 from the [manual installation](#manual-installation) section for configuration and usage.

---

## Available Commands

Run the following command to see all available options:

```bash
./city2tabula -help
```

For usage examples, refer to Database Commands documentation.

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

> [!NOTE]
> This tool is under active development. Features and performance may evolve with future releases.

## Acknowledgments

This project is being developed in the context of the research project RENvolveIT (https://projekte.ffg.at/projekt/5127011).

This research was funded by CETPartnership, the Clean Energy Transition Partnership under the 2023 joint call for research proposals, co-funded by the European Commission (GA N°101069750) and with the funding organizations detailed on https://cetpartnership.eu/funding-agencies-and-call-mod-ules.​

 <img src="docs/source/img/CETP-logo.svg" alt="CETPartnership" width="144" height="72">  <img src="docs/source/img/EN_Co-fundedbytheEU_RGB_POS.png" alt="EU" width="180" height="40">

- **3DCityDB**: For providing the foundation for 3D spatial data management. (accessed 13.11.2025, [https://www.3dcitydb.org/](https://github.com/3dcitydb))
- TABULA & EPISCOPE (IEE Projects): building-characteristic data (accessed 13.11.2025, [https://episcope.eu/iee-project/tabula/](https://episcope.eu/iee-project/tabula/)
