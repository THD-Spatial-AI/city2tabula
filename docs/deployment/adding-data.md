---
audience: developer
---

# Adding data to a running deployment

A country's database grows by adding source files under `data/lod2/<country>/` and importing them. The import can run on a schedule: a run over unchanged files does nothing.

```bash
COUNTRY=germany c2t -create-db     # once per country: creates the database and imports every folder
COUNTRY=germany c2t -import-data   # afterwards, as often as needed: imports what changed
```

`-import-data` only imports. Feature extraction and the PyLovo link run when a region is requested (`POST /api/v1/runs`), or with `-extract-features` and `-link-pylovo`.

## What an import does

| Folder state since the last import | Result |
|---|---|
| No files added, replaced or removed | citydb-tool is not run for the folder |
| Files added or replaced | The folder is imported again; buildings already in the database are skipped, so none is duplicated |
| New dataset folder | The folder is imported |

A folder counts as changed when the path, size or modification time of any file in its `gml/` or `cityjson/` folder changes. A run for a bbox (`POST /api/v1/runs`, or `-import-data -bbox`) after a whole-folder import of unchanged files does not run citydb-tool either.

## Importing one region

A run for a bbox imports only the files whose declared extent intersects it: a CityGML file's `boundedBy` envelope, or a CityJSON file's `metadata.geographicalExtent` in its `referenceSystem`. A file without a readable extent, or with a CRS that is not recognised (see [Checking files before publishing](#checking-files-before-publishing)), is always passed to citydb-tool, whose own bbox filter then applies. The bbox is recorded against the selected files alone, so:

| Since the last run for the same bbox | Result |
|---|---|
| No selected file added, replaced or removed | citydb-tool is not run |
| Files added or changed in another region | citydb-tool is not run |
| A selected file added, replaced or removed | The selected files are imported again; existing buildings are skipped |

With all 18,297 Bavarian tiles (142 GB) in one dataset folder, a region of about 2 by 1 km selects 4 files, imports in 9 s, and a repeat run takes about 1 s.

!!! warning "One invalid attribution file stops the country's import"
    Before anything is imported, every dataset folder of the country is checked: it needs a valid `attribution.json` ([Dataset attribution file](../code/attribution/index.md)) and a `gml/` or `cityjson/` folder, and no model file may lie directly in `data/lod2/<country>/`. One failing folder stops the whole import for that country, with an error naming the folder; nothing is imported until it is fixed. A scheduled job should report the error.

## Checking files before publishing

`-check-data` checks a staged dataset folder for the country without importing it or connecting to a database. It needs only `COUNTRY`.

```bash
COUNTRY=germany c2t -check-data data/lod2/germany/.bavaria-upload
```

| Check | Fails when |
|---|---|
| `attribution.json` | It is missing or breaks a rule of the [attribution file](../code/attribution/index.md) |
| Model folders | There is neither a `gml/` nor a `cityjson/` folder, or a model file lies outside them (hidden folders are not read) |
| `dataset_id` | A published folder of the country uses it with a different `attribution.json`. The same file means the staged files extend that dataset |
| Model files | A file, gzipped or not and in any subfolder, is not CityGML or CityJSON, or its declared CRS is not the SRID the import uses: `CITYDB_SRID`, or the country's default ([Configuration](../configuration/index.md)) |

The CRS is read from a CityGML file's `srsName` values in its first MiB and from a CityJSON file's `metadata.referenceSystem`. EPSG names are accepted in the `urn:ogc:def:crs:EPSG::<code>`, `https://www.opengis.net/def/crs/EPSG/0/<code>` and `EPSG:<code>` forms, as are the AdV names `urn:adv:crs:ETRS89_UTM32*` and `urn:adv:crs:ETRS89_UTM33*` and the compound EPSG:7415 (Amersfoort / RD New + NAP). Any other name, such as `unknown`, fails.

The command lists every problem, each naming its file or folder, and exits with status 1; a folder that passes prints `ready to publish` and exits with status 0.

## Staging files

A file that is still being written must not be in a model folder when an import runs. Write files outside the model folders, then rename them into place on the same filesystem, where a rename is atomic. No marker file is needed.

| Adding | Steps |
|---|---|
| A new dataset | Write the folder under a hidden name, e.g. `data/lod2/germany/.bavaria-upload/` with `attribution.json` and `gml/`, then rename it to `data/lod2/germany/bavaria/`. Hidden folders are not read. |
| Files to an existing dataset | Write each file to `data/lod2/germany/.staging/`, then rename it into `data/lod2/germany/bavaria/gml/`. |

A file renamed into place during an import is picked up by the next one.

## Limitations

- A provider's corrected tile is not applied: buildings already in the database are skipped on re-import, and their extracted features are kept.
- An import with `IMPORT_LIMIT` set is partial by design and is never skipped.
