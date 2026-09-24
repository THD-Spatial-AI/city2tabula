---
audience: developer
---

# Data

## Example Datasets

City2TABULA has been tested with the following datasets:

### LOD2 (Level of Detail 2)

| Country | Region | Format | Source | License |
| ------- | ------ | ------ | ------ | ------- |
| Germany | Deggendorf, Bavaria | CityGML | Bayerische Vermessungsverwaltung - [www.geodaten.bayern.de](https://www.ldbv.bayern.de/) | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/deed.de) |
| Austria | Vienna | CityGML | Stadt Wien - [data.wien.gv.at](https://www.data.gv.at/auftritte/?organisation=stadt-wien) | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/deed.de) |
| Netherlands | Loenen | CityJSON | © 3DBAG by tudelft3d and 3DGI - <https://docs.3dbag.nl/en/copyright/> | [CC BY 4.0](http://creativecommons.org/licenses/by/4.0/) |

### LOD3 (Level of Detail 3)

| Country | Region | Format | Source | License | Notes |
| ------- | ------ | ------ | ------ | ------- | ----- |
| Germany | Hamburg | CityGML + Textures | [MetaVer Geodata Portal](https://metaver.de/trefferanzeige?docuuid=B438AD57-223B-43A4-8E74-767CEC8A96D7#detail_links) | [Data licence Germany – attribution – Version 2.0](http://www.govdata.de/dl-de/by-2-0) | Includes building textures and detailed geometries |

!!! note "Licensing and Attribution"
    These datasets are examples for testing and development, taken from publicly available sources. Check the original source for current licensing and attribution requirements before using a dataset elsewhere.

    All dataset links were last accessed on 2026-03-13

## TABULA Building Typology Data

The TABULA building typology data included in this repository (data/tabula/ and testdata/*/seed_tabula_variant.sql)

**Source:** IEE Projects TABULA + EPISCOPE ([www.episcope.eu](https://www.episcope.eu))

## File Formats Supported

| Format | Extension | Description |
| ------- | --------- | ----------- |
| CityGML | `.gml` | Following [CityGML specification](https://www.ogc.org/standards/citygml) |
| CityJSON | `.json` | Following [CityJSON specification](https://www.cityjson.org/) |
| CSV | `.csv` | Comma-separated values, used for TABULA building typology data |

??? question "How to download bulk files from .meta4 files?"
    Some geo-portals provide metadata files in [META4](https://file.org/extension/meta4) format, which hold links to the actual data files. A download manager that supports `.meta4`, such as [aria2](https://aria2.github.io/), fetches them.

    1. Install aria2, following the [aria2 GitHub page](https://github.com/aria2/aria2).
    2. Download the `.meta4` file from the geo-portal.
    3. Download every file it links, into the current directory:

    ```bash
    aria2c <file>.meta4
    ```

    These datasets are large, so check the available disk space first.
