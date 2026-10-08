---
audience: developer
---

# Troubleshooting

Errors are listed by the step that reports them. Setting `LOG_LEVEL=DEBUG` also logs each SQL task as it starts and finishes.

## Configuration

| Error | Cause | Fix |
|-------|-------|-----|
| `Invalid configuration: missing required environment variables: ...` | `.env` is missing, or the listed keys are empty. | Copy `.env.example` to `.env` and set the listed keys. See [Configuration](../configuration/index.md). |
| `unsupported country "<name>": no TABULA data available` | `COUNTRY` is not one of the 20 countries with TABULA data. | Use a country listed in `.env.example`. |
| `citydb executable not found at <path> (set CITYDB_TOOL_PATH to the citydb-tool directory)` | `CITYDB_TOOL_PATH` points somewhere other than the citydb-tool directory. | Set it to the directory that holds the `citydb` executable. The Docker image sets it already. |
| `-relink only applies together with -link-pylovo` | `-relink` was given alone. | Run `c2t -link-pylovo -relink`. |

## Database connection

| Error | Cause | Fix |
|-------|-------|-----|
| `connect to bootstrap DB failed: ...` | City2TABULA cannot reach the `postgres` database on `DB_HOST:DB_PORT`, which it uses to check for and create the target database. | Check that PostgreSQL runs, and check `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD` and `DB_SSL_MODE`. |
| `failed to create database <name>: ... permission denied` | `DB_USER` may not create databases. | Grant `CREATEDB` to the role, or create the database by hand under the suffixed name (`<DB_NAME>_<country code>`). |
| `failed to enable PostGIS: ...` | The PostGIS extension is not installed on the server, or the role may not create extensions. | Install PostGIS 3 on the server, or create the extension once as a superuser. |

## Creating the database and importing data

| Error | Cause | Fix |
|-------|-------|-----|
| `Database already exists.` on `-create-db` | The database already has the CityDB schemas. | To add data, run `c2t -import-data` and then `c2t -extract-features`; both skip what is already done. To start again, `c2t -reset-db` drops and rebuilds everything, including hand corrections. |
| `<file> lies directly in <folder>; move it into a dataset folder holding an attribution.json` | A model file sits in `data/lodN/<country>/` instead of a dataset folder. | Move it to `data/lodN/<country>/<dataset>/gml/` or `cityjson/`. See [Data](../example/example.md). |
| `dataset folder <folder> has neither a gml nor a cityjson subfolder holding its model files` | The dataset folder has no `gml/` or `cityjson/` folder. | Put the CityGML files in `gml/` and the CityJSON files in `cityjson/`. |
| `read attribution file: open <folder>/attribution.json: no such file or directory` | The dataset folder has no attribution file. | Add one. See [Dataset attribution file](../code/attribution/index.md). |
| `<folder>/attribution.json: ...` | The attribution file breaks a rule. | Fix every rule listed. |
| `dataset_id "<id>" is used by both <file> and <file>` | Two attribution files share a `dataset_id`. | Give each dataset its own `dataset_id`. |

## Feature extraction and the API

| Error | Cause | Fix |
|-------|-------|-----|
| `... have a lineage with no dataset_attribution row ...; run -sync-attribution` | Buildings were imported from a dataset whose attribution was never stored, or was removed. | Run `c2t -sync-attribution`. It re-reads every `attribution.json` without re-importing. |
| `<database> has no dataset_attribution row for some of [...]; run -sync-attribution` | The API found a building whose dataset has no credit stored. | Run `c2t -sync-attribution` against that database. |
| `task <name> failed (SQL file: <file>): ...` followed by `<n> job(s) failed` | An SQL script failed for a batch of buildings. The other batches complete. | Read the PostgreSQL error in the log. After fixing the cause, run `-extract-features` again: buildings already processed are skipped. |
| `task <name> failed after <n> deadlock retries` | Batches kept locking the same rows. | Run again with a lower `THREAD_COUNT`. |
| `PyLovo FDW setup failed at "<step>": ...` | The PostgreSQL server cannot reach the PyLovo database, or the PyLovo role lacks `SELECT` on `res` and `oth`. The server, not City2TABULA, opens this connection. | Check that `PYLOVO_FDW_HOST` resolves from the database server and that the role can read both tables. See [PyLovo building link](../code/pylovo-link/index.md). |
