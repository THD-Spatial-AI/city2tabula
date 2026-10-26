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

!!! warning "One invalid attribution file stops the country's import"
    Before anything is imported, every dataset folder of the country is checked: it needs a valid `attribution.json` ([Dataset attribution file](../code/attribution/index.md)) and a `gml/` or `cityjson/` folder, and no model file may lie directly in `data/lod2/<country>/`. One failing folder stops the whole import for that country, with an error naming the folder; nothing is imported until it is fixed. A scheduled job should report the error.

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
