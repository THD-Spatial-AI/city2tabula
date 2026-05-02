# Setup and Usage

For using the City2TABULA tool, you have two main options: the recommended Docker-based setup for ease of use and consistency, or a manual installation for advanced users who prefer direct control over the environment for development purposes.

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

Place your 3D city data file (.gml or .json) under `data/` directory before starting the containers:

```bash
data/
├── lod2/<country>/*(.gml | .json)
└── lod3/<country>/*(.gml | .json)
```

!!! example
    For example, if you have a LoD2 CityGML file for Germany, you would place it in `data/lod2/germany/` directory. If you have a corresponding LoD3 file, it would go in `data/lod3/germany/`. The directory structure should look like this:

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
    If you don’t have your own data, you can use the example datasets provided in the `data/` directory. These are sourced from publicly available datasets with appropriate licensing. Refer to this [documentation](https://github.com/thd-spatial-ai/city2tabula/blob/main/data/README.md) for example datasets and sources.

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

### Step 4. Create database
After the setup is complete, you can create the database with:

```bash
# Linux/macOS
make create-db

# Windows (Command Prompt)
setup.bat create-db

# Windows (PowerShell)
./setup.ps1 create-db
```

!!! note
    If you have already created the database, you will need to change the database name or reset the existing database before running the above command again.
    To change the database name update environment configuration in `docker.env` file. To reset the database, use the following command:

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

Final step is to run the feature extraction process, which will execute the full City2TABULA pipeline and generate the output data in the database. Use the following command:

```bash

# Linux/macOS
make extract-features

# Windows (Command Prompt)
setup.bat extract-features

# Windows (PowerShell)
./setup.ps1 extract-features
```

## Development Setup

!!! warning
    This setup is mainly intended for Linux development environments. If you’re on Windows, Docker is strongly recommended. Local installation on Windows might require additional configuration (e.g., WSL2, manual Java setup) and is not covered in this guide.

### Prerequisites (dev)

| Requirement | Version | Notes | Download Link |
| ----------- | ------- | ----- | ------------- |
| Go          | 1.21+   | required for City2TABULA | [golang.org](https://go.dev/doc/install)                                                           |
| PostgreSQL                 | 17+     | required for City2TABULA & CityDB Tool     | [postgresql.org](https://www.postgresql.org/download/)                                             |
| PostGIS                    | 3.5+    | required for working with spatial data | [postgis.net](https://postgis.net/install/)                                                        |
| Java                       | 17+  | required for CityDB Tool| [oracle.com](https://www.oracle.com/java/technologies/downloads/)                                  |
| Git                        | 2.25+   | required for City2TABULA | [git-scm.com](https://git-scm.com/downloads)                                                       |
| CityDB Importer/Exporter   | v1.1.0 | Unzip and place the `citydb-tool` directory at your preferred location. | [github.com](https://github.com/3dcitydb/citydb-tool/releases/tag/v1.1.0)                          |

!!! note
    Steps for local development setup will be added in future updates. For now, refer to the Docker setup instructions for a streamlined installation process.