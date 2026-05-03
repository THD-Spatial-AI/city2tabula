# Data

## Example Datasets

Tool has been tested with the following datasets:

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
    The above datasets are provided as examples for testing and development purposes. They are sourced from publicly available datasets with appropriate licensing. Always check the original sources for the most up-to-date licensing information and attribution requirements when using these datasets in your projects.

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
    Some geo-portals provide metadata files in [META4](https://file.org/extension/meta4) format, which contain links to the actual data files. To download the bulk files, you can use a download manager that supports .meta4 files, such as [aria2](https://aria2.github.io/).

    **Here's how you can do it:**

    1. Install aria2 if you haven't already. You can find installation instructions on the [aria2 GitHub page](https://github.com/aria2/aria2).

    2. Download the .meta4 file from the geo-portal.

    3. Use the following command to start downloading the files listed in the .meta4 file:
    ```bash
    aria2c <yourfile>.meta4
    ```
    Replace `<yourfile>.meta4` with the actual name of your .meta4 file. This command will read the .meta4 file and `download all the linked files to your current directory`. **Make sure you have enough storage space and a stable internet connection, as the files can be quite large.**
