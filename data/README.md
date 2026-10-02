# City2TABULA Data Directory

This directory contains input datasets for the City2TABULA pipeline.

## Directory Structure

```
data/
├── lod2/          # LoD2 CityGML / CityJSON input files (per country)
├── lod3/          # LoD3 CityGML / CityJSON input files (per country)
└── tabula/        # TABULA building typology reference data (CSV, one file per country)
```

## TABULA Building Typology Data

The CSV files under `data/tabula/` are derived from the **TABULA** (Typology Approach for
Building Stock Energy Assessment) project, a European initiative coordinated under the
IEE (Intelligent Energy Europe) programme.

**Source:** IEE Projects TABULA + EPISCOPE ([www.episcope.eu](https://www.episcope.eu))

`data/tabula/attribution.json` carries this credit. City2TABULA stores it with the 3D datasets'
credits.

**Reference:**

> Loga, T., Stein, B., Diefenbach, N. (2016). *TABULA building typologies in 20 European
> countries – making energy-related features of residential building stocks comparable.*
> Energy and Buildings, 132, 4–12. https://doi.org/10.1016/j.enbuild.2016.06.094

## LoD2 / LoD3 Building Data

CityGML and CityJSON files placed under `data/lod2/` and `data/lod3/` are user-supplied, one
folder per source dataset: `data/lod2/<country>/<dataset>/` holds an `attribution.json` crediting
the provider, a `gml/` folder for CityGML and a `cityjson/` folder for CityJSON. Nothing else in a
dataset folder is imported. See `docs/code/attribution/index.md`.
See the country-specific `.gitignore` files in each subdirectory; large files are excluded
from the repository. Refer to the original data provider for licensing terms.
