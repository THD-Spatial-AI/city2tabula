# Setup & Installation

For using the City2TABULA tool, you have two main options: the recommended Docker-based setup for ease of use and consistency, or a manual installation for advanced users who prefer direct control over the environment for development purposes.

## Docker (recommended)

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- Linux/macOS: `make` (usually pre-installed)
- Windows: use `setup.bat` for Command Prompt or `setup.ps1` for PowerShell

### Step 1. Download release

Download the latest release from [GitHub](https://github.com/THD-Spatial/city2tabula/releases). Unzip the downloaded file and navigate to the project directory:

```bash
cd city2tabula-<version>
```

### Step 2. Download data

Place your 3D city data file (.gml or .json) under `data/` directory before starting the containers:


```text
data/
├── lod2/<country>/*(.gml | .json)
├── lod3/<country>/*(.gml | .json)
└── tabula/<country>.csv
```

!!! note
    If you don’t have your own data, you can use the example datasets provided in the `data/` directory. These are sourced from publicly available datasets with appropriate licensing. Refer to this [documentation](https://github.com/THD-Spatial/city2tabula/blob/main/data/README.md) for example datasets and sources.

### Step 3. Create Docker Container

This will build the Docker images, start the containers, and run the interactive setup script to configure environment variables and database connection settings. Follow the prompts to complete the setup.

Choose and run the appropriate command for your operating system:

```bash
# Linux/macOS
make setup

# Windows (Command Prompt)
setup.bat setup

# Windows (PowerShell)
./setup.ps1 setup
```


## Option B: Development Setup (manual installation)

> [!WARNING]
> This setup is mainly intended for Linux development environments. If you’re on Windows, Docker is strongly recommended. Local installation on Windows might require additional configuration (e.g., WSL2, manual Java setup) and is not covered in this guide.

### Dependencies

- **Go**: 1.21 or later (Download from [golang.org](https://go.dev/doc/install))

- **PostgreSQL**: 17+ with PostGIS 3.5+ (Download from [postgresql.org](https://www.postgresql.org/download/))

- **PostGIS**: 3.5+ (Download from [postgis.net](https://postgis.net/install/))

- **Java**: 17+ for CityDB Tool (Download from [oracle.com](https://www.oracle.com/java/technologies/downloads/))

- **Git**: 2.25+ for source management (Download from [git-scm.com](https://git-scm.com/downloads))

- **CityDB Importer/Exporter**: v1.1.0 (Download from [github.com](https://github.com/3dcitydb/citydb-tool/releases/tag/v1.1.0))
    - Unzip the downloaded file and place the `citydb-tool` directory in a known location (e.g., `/opt/citydb-tool` on Linux or `C:\Program Files\citydb-tool` on Windows).

### Build

Navigate to the project directory and build the Go application:

```bash
go build -o city2tabula ./cmd
./city2tabula -help
```

### Configuration

Create a `.env` in the project root directory and set at least:

- `COUNTRY`
- DB connection settings (`DB_NAME`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_SSL_MODE`)
- CityDB tool location + CRS settings (`CITYDB_TOOL_PATH`, `CITYDB_SRID`, `CITYDB_SRS_NAME`)

### Run

```bash
./city2tabula --create-db
./city2tabula --extract-features
```
