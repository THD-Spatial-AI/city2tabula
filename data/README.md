# City2TABULA Data Directory

This directory contains sample datasets for testing and development purposes. All data is sourced from publicly available datasets with appropriate licensing.

## Directory Structure

```
data/
├── lod2/                  # LOD2 production data (empty - populate as needed)
├── lod3/                  # LOD3 production data (empty - populate as needed)
└── tabula/                # TABULA reference data (No changes required)
```

## Example Datasets

### LOD2 (Level of Detail 2)

| Country | Region | Format | Source | License |
|---------|--------|--------|--------|---------|
| Germany | Deggendorf, Bavaria | CityGML | [Bayerische Vermessungsverwaltung](https://geodaten.bayern.de/opengeodata/OpenDataDetail.html?pn=lod2) | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/deed.de) |
| Austria | Vienna | CityGML | [Vienna Open Government Data](https://www.wien.gv.at/downloads/ma41/dach-lod2-gml.zip) | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/deed.de) |
| Netherlands | Loenen | CityJSON | [3DBAG](https://3dbag.nl/en/download) (Tiles: [7-736-608.city.json](https://data.3dbag.nl/v20241216/tiles/7/736/608/7-736-608.city.json), [8-736-600.city.json](https://data.3dbag.nl/v20241216/tiles/8/736/600/8-736-600.city.json)) | [CC BY 4.0](http://creativecommons.org/licenses/by/4.0/) |

### LOD3 (Level of Detail 3)

| Country | Region | Format | Source | License | Notes |
|---------|--------|--------|--------|---------|-------|
| Germany | Hamburg | CityGML + Textures | [MetaVer Geodata Portal](https://metaver.de/trefferanzeige?docuuid=B438AD57-223B-43A4-8E74-767CEC8A96D7#detail_links) | [Data licence Germany – attribution – Version 2.0](http://www.govdata.de/dl-de/by-2-0) | Includes building textures and detailed geometries |

> [!NOTE]
>All datasets accessed on 2026-02-09

## Data Usage

### For Development

```bash
# Use example data for testing
./city2tabula --create-db  # Creates database with example data
./city2tabula --extract-features  # Processes example datasets
```

### For Production

1. Place your CityGML/CityJSON files in the appropriate `lod2/` or `lod3/` directories
2. Update the configuration to point to your data sources
3. Run the pipeline with your production data

## File Formats Supported

| Format | Extension | Description |
|--------|-----------|-------------|
| CityGML | `.gml` | OGC CityGML format |
| CityJSON | `.json` | JSON-based encoding of CityGML |

## Licensing & Attribution

All example datasets are provided under open licenses. When using this data:

1. **Attribute the original source** when publishing results
2. **Respect license terms** (CC BY 4.0, Data licence Germany, etc.)
3. **Check original sources** for the most up-to-date licensing information

## Notes

- Example data is provided for **testing and development only**
- File sizes are optimized for Git LFS to keep repository lightweight
- Production data directories (`lod2/`, `lod3/`, `tabula/`) are initially empty

## Downloading data from .meta4 files

If you have `.meta4` files for datasets, you can use a download manager that supports Metalink files, such as `aria2`, to download the actual data files.

```bash
sudo apt install aria2  # Install aria2 if not already installed
aria2c -M path/to/yourfile.meta4  # Download files using the .meta4 file
```
