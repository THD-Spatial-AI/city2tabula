# Changelog

Each release is listed with its breaking changes first. Releases before
v0.8.0 are described in their tag messages.

## Unreleased

Features:
- GET /api/v1/buildings serves each surface's length, width and height,
  and the OpenAPI Surface schema defines how every surface value is
  measured, with references.

## v0.8.0 (2026-10-03)

Breaking changes:
- GET /api/v1/buildings and GET /api/v1/geometry return
  {"buildings": [...], "attributions": [...]} instead of a bare array.
  Each building carries dataset_id; attributions holds the credit of
  every dataset returned, plus TABULA's when a building has a TABULA
  type (#162).
- Source data sits in one folder per dataset,
  data/lodN/<country>/<dataset>/, holding an attribution.json and gml/
  or cityjson/ folders with the model files. A model file directly in a
  country folder, or a dataset folder without a valid attribution.json,
  stops the import (#159).
- Databases built by an earlier release have no dataset_attribution
  table, dataset_id column or feature lineage, and must be re-created
  and re-imported (#159).

Features:
- Dataset attribution: attribution.json format, validator and JSON
  Schema (#158); a dataset_attribution table, a NOT NULL dataset_id on
  every building from the import lineage, and -sync-attribution to
  correct a credit without re-importing (#159).
- Per-surface in-plane length and width (#142).
- -link-pylovo -relink re-links buildings after the PyLovo data
  changed (#155).

Fixes:
- BuildingParts are aggregated into one row per Building (#150).
- Buildings extracted at LOD3 are served and linked (#147).
- Buildings with only ground faces are no longer output (#157).
- Walls of concave footprints face outward; their azimuth was 180
  degrees off (#165).
- Normals, and so tilt, azimuth and area, of faces with holes are
  correct and repeatable (#163).
- -link-pylovo puts each building in one batch (#164) and pushes its
  PyLovo pre-filter down over postgres_fdw (#153).
- The server host port is read from HOST_PORT (#145).
