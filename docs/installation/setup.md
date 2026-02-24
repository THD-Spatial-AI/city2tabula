# Setup & Installation

This page covers the recommended Docker workflow and an advanced manual installation.

## Option A: Docker (recommended)

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- Linux/macOS: `make` (usually pre-installed)
- Windows: use `setup.bat` or `setup.ps1`

### Step 1. Clone the repository

```bash
git clone https://github.com/THD-Spatial/City2TABULA.git
cd City2TABULA
```

### Step 2. Run interactive setup

```bash
# Linux/macOS
make setup

# Windows (Command Prompt)
setup.bat setup

# Windows (PowerShell)
./setup.ps1 setup
```

### Step 3. Enter the dev shell

```bash
# Linux/macOS
make dev

# Windows (Command Prompt)
setup.bat dev

# Windows (PowerShell)
./setup.ps1 dev
```

### Step 4. Run the pipeline (inside the container)

```bash
./city2tabula --create-db
./city2tabula --extract-features
```

### Step 5. Data layout

Place your data under `data/` before starting the containers:

```text
data/
├── lod2/<country>/*.(gml|json)
├── lod3/<country>/*.(gml|json)
└── tabula/<country>.csv
```

## Option B: Manual installation (advanced)

> This is mainly intended for Linux development environments. If you’re on Windows, Docker is strongly recommended.

### Dependencies

- **Go**: 1.21 or later (Download from [golang.org](https://go.dev/doc/install))

- **PostgreSQL**: 17+ with PostGIS 3.5+ (Download from [postgresql.org](https://www.postgresql.org/download/))

- **PostGIS**: 3.5+ (Download from [postgis.net](https://postgis.net/install/))

- **Java**: 17+ for CityDB Tool (Download from [oracle.com](https://www.oracle.com/java/technologies/downloads/))

- **Git**: 2.25+ for source management (Download from [git-scm.com](https://git-scm.com/downloads))

- **CityDB Importer/Exporter**: v1.1.0 (Download from [github.com](https://github.com/3dcitydb/citydb-tool/releases/tag/v1.1.0))
    - Unzip the downloaded file and place the `citydb-tool` directory in a known location (e.g., `/opt/citydb-tool` on Linux or `C:\Program Files\citydb-tool` on Windows).

### Build

```bash
go build -o city2tabula ./cmd
./city2tabula --help
```

### Configuration

Create a `.env` (or use your preferred config mechanism used by the project) and set at least:

- `COUNTRY`
- DB connection settings (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_SSL_MODE`)
- CityDB tool location + CRS settings (`CITYDB_TOOL_PATH`, `CITYDB_SRID`, `CITYDB_SRS_NAME`)

### Run

```bash
./city2tabula --create-db
./city2tabula --extract-features
```
