---
audience: developer
---

# Setup and Usage

There are two ways to run City2TABULA: the Docker setup, which is the recommended one, and a manual installation for development work that needs direct control over the environment.

## Docker Setup (recommended)

### Prerequisites (Docker)

| Requirement         | Version | Notes | Download Link |
| ------------------- | ------- | ----- | ------------- |
| Docker              | 20.10+  |       | [docker.com](https://www.docker.com/get-started) |
| Docker Compose      | 2.0+    |       | [docs.docker.com](https://docs.docker.com/compose/install/) |

### Step 1. Download release

Download the latest release from [GitHub](https://github.com/thd-spatial-ai/city2tabula/releases). Unzip the downloaded file and navigate to the project directory:

```bash
cd city2tabula-<version>
```

### Step 2. Download data

Place the 3D city data files (`.gml` or `.json`) under `data/` before starting the containers:

```bash
data/
├── lod2/<country>/*(.gml | .json)
└── lod3/<country>/*(.gml | .json)
```

!!! example
    A LoD2 CityGML file for Germany goes in `data/lod2/germany/`, and a corresponding LoD3 file in `data/lod3/germany/`:

    ```bash
    data/
    ├── lod2/
    │   └── germany/
    │       └── germany_lod2.gml
    └── lod3/
        └── germany/
            └── germany_lod3.gml
    ```



!!! tip
    Subdirectories within a country directory are allowed, for example `data/lod2/germany/berlin/` and `data/lod2/germany/munich/`. Files must sit under the correct LoD and country directory to be processed.

    The [Data](../example/example.md) page lists sources to download from.

### Step 3. Create Docker Container

This builds the Docker images, starts the containers, and runs the interactive setup script for the environment variables and database connection settings. The script prompts for each value.

Run the command for the operating system in use:

```bash
# Linux/macOS
make setup

# Windows (Command Prompt)
setup.bat setup

# Windows (PowerShell)
./setup.ps1 setup
```

### Step 4. Create database

After setup completes, create the database:

```bash
# Linux/macOS
make create-db

# Windows (Command Prompt)
setup.bat create-db

# Windows (PowerShell)
./setup.ps1 create-db
```

!!! note
    Running this against an existing database requires either a different database name or a reset first. The database name is set in `docker.env`. To change it:

    ```bash
    # Linux/macOS
    make configure

    # Windows (Command Prompt)
    setup.bat configure

    # Windows (PowerShell)
    ./setup.ps1 configure
    ```

    To reset the database, use:

    ```bash
    # Linux/macOS
    make reset-db

    # Windows (Command Prompt)
    setup.bat reset-db

    # Windows (PowerShell)
    ./setup.ps1 reset-db
    ```

### Step 5. Run feature extraction

The final step runs the extraction pipeline and writes the output data to the database:

```bash

# Linux/macOS
make extract-features

# Windows (Command Prompt)
setup.bat extract-features

# Windows (PowerShell)
./setup.ps1 extract-features
```

## Development Setup

!!! warning "Only for Unix-based systems (Linux/macOS)"
    This setup targets Linux development environments. On Windows, use the Docker setup: a local installation there needs additional configuration (WSL2, a manual Java setup) that this guide does not cover.

### Prerequisites (dev)

| Requirement | Version | Notes | Download Link |
| ----------- | ------- | ----- | ------------- |
| Go | 1.25+ | Builds City2TABULA | [golang.org](https://go.dev/doc/install) |
| PostgreSQL | 15 to 18 | Used by City2TABULA and citydb-tool | [postgresql.org](https://www.postgresql.org/download/) |
| PostGIS | 3.4+ | Spatial types and functions | [postgis.net](https://postgis.net/install/) |
| Java | 17+ | Required by citydb-tool | [oracle.com](https://www.oracle.com/java/technologies/downloads/) |
| Git | 2.25+ | Clones the repository | [git-scm.com](https://git-scm.com/downloads) |
| citydb-tool | v1.3.2 | Unzip and place the `citydb-tool` directory anywhere; its path goes in `.env` | [github.com](https://github.com/3dcitydb/citydb-tool/releases) |

### Step 1. Clone the repository

```bash
git clone https://github.com/thd-spatial-ai/city2tabula.git
cd city2tabula
```

### Step 2. Install PostgreSQL, PostGIS and the CityDB tool

Install the versions listed in the prerequisites table. Unzip the citydb-tool release and note the path to the extracted `citydb-tool-<version>` directory, which Step 4 needs.

### Step 3. Build the binary

```bash
go build -o c2t ./cmd/c2t
```

### Step 4. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and set at minimum:

```bash
COUNTRY              # e.g. germany, must match one of the supported TABULA/EPISCOPE countries
DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME  # the local PostgreSQL instance
CITYDB_TOOL_PATH     # path to the citydb-tool directory from Step 2 (not needed in Docker, where it is in the image)
```

`CITYDB_SRID` and `CITYDB_SRS_NAME` are derived from `COUNTRY`. Set them only for a country missing from that lookup table (`internal/config/srid.go`).

`DB_NAME` is a base name. The tool appends the country's ISO 3166-1 alpha-2 code (`DB_NAME=city2tabula` with `COUNTRY=netherlands` gives `city2tabula_nl`) and creates the database if it is absent. To use an existing database, give that database the suffixed name.

### Step 5. Add the data

Place the 3D city data files under `data/`, in the same layout as the Docker setup:

```bash
data/
├── lod2/<country>/*(.gml | .json)
└── lod3/<country>/*(.gml | .json)
```

### Step 6. Create the database and run the pipeline

```bash
./c2t -create-db          # creates CityDB and City2TABULA schemas, then imports the data
./c2t -extract-features   # runs the feature extraction pipeline
```

`./c2t -help` lists every flag, including `-reset-db` and `-link-pylovo`.
