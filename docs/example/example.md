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

## Level of detail and geometry

LoD2 data goes in `data/lod2/`, LoD3 data in `data/lod3/`. citydb-tool imports each into its own schema (`lod2`, `lod3`), and the pipeline writes one set of output tables per LoD (`city2tabula.lod2_building`, `city2tabula.lod3_building` and their surface tables).

Each CityGML `Building` gives one output row. Its geometry is every solid owned by the Building or by a `BuildingPart` below it, at any depth. From each solid owner the pipeline takes the boundary surfaces that carry `lodNMultiSurface` geometry of the schema's LoD, as MultiPolygons. A source that ships several LoDs per part, such as 3DBAG with LoD 1.2, 1.3 and 2.2, contributes only the matching LoD.

| Read | Not read |
|------|----------|
| `WallSurface`, `RoofSurface` and `GroundSurface`, which all measures use | `BuildingInstallation` surfaces (dormers, chimneys, balconies), which sit on a closed envelope |
| Other boundary surfaces, such as `ClosureSurface`, carried without area | Openings (windows, doors) |
| | Surface geometry that is not a MultiPolygon |
| | Textures and appearances |

A building needs `GroundSurface` faces: they give its footprint and the inside point that orients its walls. Storeys, roof type and the other building features are derived from the geometry. Attributes in the source file, such as a stored storey count, are not used.

### BuildingPart patterns

Sources differ in which features own the solids and in how they model the wall between two parts of one building. The pipeline handles each of the patterns below. Where two parts touch, the shared face region is internal and is left out of the envelope.

| Source | Solid owner | Wall between parts |
|--------|-------------|--------------------|
| Bremen LoD2 | The Building | No parts |
| 3DBAG | One BuildingPart, `<id>-0`, in almost every building | Rarely applies |
| Vienna LoD2 | The Building when it has no parts, otherwise each of its parts | Modelled on both sides |
| Prague LoD3 | The Building and each of its parts | Omitted on both sides |

### Known limits

- One source surface feature can hold several polygons. Each polygon is its own row with the source feature's id, so `surface_feature_id` is not unique per face. The surface `id` is.
- A wall shared with another building is detected only when that building is in the same database. See [Party walls](../code/sql-pipeline/post-02-party-walls.md).

??? question "How to download bulk files from .meta4 files?"
    Some geo-portals provide metadata files in [META4](https://file.org/extension/meta4) format, which hold links to the actual data files. A download manager that supports `.meta4`, such as [aria2](https://aria2.github.io/), fetches them.

    1. Install aria2, following the [aria2 GitHub page](https://github.com/aria2/aria2).
    2. Download the `.meta4` file from the geo-portal.
    3. Download every file it links, into the current directory:

    ```bash
    aria2c <file>.meta4
    ```

    These datasets are large, so check the available disk space first.
