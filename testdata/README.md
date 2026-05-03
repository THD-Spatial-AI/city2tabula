# Test Data

This directory contains small seed datasets used by the City2TABULA integration tests.
Each country subdirectory holds SQL files exported from a real 3DCityDB import,
trimmed to a few hundred buildings so they are fast to load and small enough to commit to git.

For data sources and licence information see [NOTICE](../NOTICE) in the project root.

## Structure

```
testdata/
├── germany/
│   ├── seed_lod2.sql            — lod2 schema + ~355 buildings (pg_dump --schema=lod2)
│   ├── seed_tabula_variant.sql  — TABULA variant reference data for Germany
│   └── .gitignore               — excludes large local benchmark dumps
├── austria/
│   ├── seed_lod2.sql
│   ├── seed_tabula_variant.sql
│   └── .gitignore
└── netherlands/
    ├── seed_lod2.sql
    ├── seed_tabula_variant.sql
    └── .gitignore
```

## Running integration tests

```bash
# All countries
go test -tags integration -v ./internal/process/

# Single country
go test -tags integration -v -run TestPipeline_Germany_LOD2 ./internal/process/
```

Tests for a country are **automatically skipped** if its seed files are not present,
so adding a new country only requires dropping the seed files in the right directory.

## Regenerating seed files

To export a fresh seed from a CityDB database (replace `YOUR_DB` with the database name):

```bash
# Full lod2 schema — DDL + data (~50–500 buildings recommended for CI seeds)
pg_dump --host=localhost --username=postgres --dbname=YOUR_DB \
  --schema=lod2 --no-owner --no-privileges \
  --file=testdata/<country>/seed_lod2.sql

# TABULA variant data only (no DDL — table is created by RunCity2TabulaDBSetup)
pg_dump --host=localhost --username=postgres --dbname=YOUR_DB \
  --table=city2tabula.tabula_variant --data-only --disable-triggers \
  --no-owner --no-privileges \
  --file=testdata/<country>/seed_tabula_variant.sql

# Strip PostgreSQL 18 restriction tokens (if dumped from PG 18)
sed -i '/^\\restrict/d;/^\\unrestrict/d' testdata/<country>/seed_lod2.sql
sed -i '/^\\restrict/d;/^\\unrestrict/d' testdata/<country>/seed_tabula_variant.sql
```

## Attribution and Licensing

See [NOTICE](../NOTICE) in the project root for detailed information on data sources, licensing, and attribution requirements for the datasets included in this repository.