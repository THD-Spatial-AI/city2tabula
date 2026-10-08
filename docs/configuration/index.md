---
audience: developer
---

# Configuration

City2TABULA reads its configuration from environment variables. On start it loads `.env` from the working directory, and a value there overrides the same variable set in the shell. `.env.example` is the template:

```bash
cp .env.example .env
```

An empty value counts as unset and takes the default.

## Location and database

| Key | Default | Meaning |
|-----|---------|---------|
| `COUNTRY` | required | Country of the data, e.g. `germany`, `netherlands`, `united_kingdom`. Case, spaces and hyphens are normalised. Must be one of the 20 countries with TABULA data listed in `.env.example`. Selects the TABULA typology, the data folders `data/lod2/<country>/` and `data/lod3/<country>/`, and the default SRID. |
| `DB_NAME` | required | Base name of the database. The country's ISO 3166-1 alpha-2 code is appended: `city2tabula` with `COUNTRY=netherlands` gives `city2tabula_nl`. The database is created when it does not exist. |
| `DB_HOST` | `localhost` | PostgreSQL host. |
| `DB_PORT` | `5432` | PostgreSQL port. |
| `DB_USER` | `postgres` | PostgreSQL role. Creating the database needs `CREATEDB` or superuser. |
| `DB_PASSWORD` | required | Password of `DB_USER`. |
| `DB_SSL_MODE` | `prefer` | libpq `sslmode`: `disable`, `allow`, `prefer`, `require`, `verify-ca` or `verify-full`. |

## CityDB

| Key | Default | Meaning |
|-----|---------|---------|
| `CITYDB_TOOL_PATH` | required outside Docker | Directory of the citydb-tool installation, the one holding the `citydb` executable. The Docker image sets it. |
| `CITYDB_SRID` | from `COUNTRY` | EPSG code of the source data, e.g. `25832`. Set it only when the data uses another CRS than the country default in `internal/config/srid.go`. |
| `CITYDB_SRS_NAME` | from `COUNTRY` | Name of that CRS, e.g. `ETRS89 / UTM zone 32N`. Set it together with `CITYDB_SRID`. |

## Run control

| Key | Default | Meaning |
|-----|---------|---------|
| `THREAD_COUNT` | number of CPUs | Worker count for extraction and the database connection pool size. Values are clamped to between 1 and the number of CPUs. |
| `BUILDING_LIMIT` | `0` (all) | Processes at most this many buildings per LoD in `-extract-features`, and in total in `-link-pylovo`. For tests and benchmarks on an imported database. |
| `IMPORT_LIMIT` | `0` (all) | Imports at most this many features from each `gml/` or `cityjson/` folder. For building a small test database. |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN` or `ERROR`. |

## Building features

| Key | Default | Meaning |
|-----|---------|---------|
| `STOREY_HEIGHT` | `2.8` | Floor-to-floor height in metres for counting the storeys below the eave. See [Storeys](../code/sql-pipeline/06-storeys.md). |

## PyLovo link

Read only by `-link-pylovo`. The [PyLovo building link](../code/pylovo-link/index.md) page describes both modes.

| Key | Default | Meaning |
|-----|---------|---------|
| `PYLOVO_SCHEMA` | `public` | Schema holding the PyLovo `res` and `oth` tables in the City2TABULA database. Ignored when `PYLOVO_FDW_HOST` is set. |
| `PYLOVO_FDW_HOST` | empty | Host of a separate PyLovo database. When set, the link runs over `postgres_fdw` and the next four keys are required, apart from the port. |
| `PYLOVO_FDW_PORT` | `5432` | Port of the PyLovo database. |
| `PYLOVO_FDW_DBNAME` | empty | Name of the PyLovo database. |
| `PYLOVO_FDW_USER` | empty | Role with `SELECT` on `res` and `oth`. |
| `PYLOVO_FDW_PASSWORD` | empty | Password of that role. |
| `PYLOVO_LINK_GRID_SIZE` | `1000` | Side length in metres of the grid cells the link is batched by. |

## HTTP server

| Key | Default | Meaning |
|-----|---------|---------|
| `SERVER_PORT` | `5000` | Port of the [HTTP API](../api.md). |

## Validation notebook

| Key | Default | Meaning |
|-----|---------|---------|
| `VALIDATION_LIMIT` | `0` (all) | Matched pairs per category that `validation/validation.ipynb` loads. See [Generating reports](../validation/report.md). |
