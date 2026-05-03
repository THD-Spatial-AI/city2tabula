# Test Data

This directory contains small seed datasets used by the City2TABULA integration tests.
Each country subdirectory holds SQL files exported from a real 3DCityDB import,
trimmed to a few hundred buildings so they are fast to load and small enough to commit to git.

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

## Data sources and citations

### Germany

Building data from the **City of Deggendorf, Bavaria, Germany**, provided as CityGML LoD2.

> Bayerische Vermessungsverwaltung (2024). *3D-Gebäudemodell LoD2 Bayern*.
> Landesamt für Digitalisierung, Breitband und Vermessung (LDBV).
> Available under the Creative Commons Attribution licence (CC BY 4.0):
> https://geodaten.bayern.de/opengeodata/OpenDataDetail.html?pn=lod2

TABULA variant reference data (`seed_tabula_variant.sql`) — source:
**IEE Projects TABULA + EPISCOPE (www.episcope.eu)**, custodian: Institut Wohnen und Umwelt (IWU).

### Austria

Building data: *[add source and licence when available]*

TABULA variant reference data (`seed_tabula_variant.sql`) — source:
**IEE Projects TABULA + EPISCOPE (www.episcope.eu)**, custodian: Austrian Energy Agency (AEA).

### Netherlands

Building data: *[add source and licence when available]*

TABULA variant reference data (`seed_tabula_variant.sql`) — source:
**IEE Projects TABULA + EPISCOPE (www.episcope.eu)**, custodian: DGMR / TNO.

### TABULA usage conditions

Any use of TABULA data (files, datasets, tools) must visibly mention
**"IEE Projects TABULA + EPISCOPE (www.episcope.eu)"** as the source.
Usage in research, theses, and software is intended; only non-exclusive use is permitted.
If you publish work using this data, notify the project at www.episcope.eu.

**Reference:**
> Loga, T., Stein, B., Diefenbach, N. (2016). *TABULA building typologies in 20 European
> countries — Making energy-related features of residential building stocks comparable.*
> Energy and Buildings, 132, 4–12. https://doi.org/10.1016/j.enbuild.2016.06.094

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
