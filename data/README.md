# City2TABULA Data Directory

This directory contains input datasets for the City2TABULA pipeline.

## Directory Structure

```
data/
├── lod2/          # LoD2 CityGML / CityJSON input files (per country)
├── lod3/          # LoD3 CityGML / CityJSON input files (per country)
├── attributes/    # Building attributes from outside the 3D model, e.g. building use (per country)
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

## Building Attributes

`data/attributes/<country>/<dataset>/` holds building attributes that the 3D model does not carry.
Each folder has an `attribution.json`, validated like a 3D dataset's, and a `building_use.csv`:

```
object_id,use_code,floor_area
NL.IMBAG.Pand.0150100000000150,woonfunctie,72
```

| Column | Content |
|---|---|
| `object_id` | The building's object id exactly as the 3D data carries it |
| `use_code` | One use code of the provider, listed in `internal/onrequest/usetype/<country>.csv` |
| `floor_area` | Floor area in m2 that the code covers, a number >= 0 |

A building with several uses has one row per code. Where its source function code gives no use,
`GET /api/v1/buildings` serves the use covering the most floor area. Each import replaces a
dataset's rows. In the Netherlands the rows come from BAG units (`verblijfsobject` gebruiksdoel
and oppervlakte); a building without units has one row with use code `geen verblijfsobject` and
floor area 0, which maps to `ancillary`.
