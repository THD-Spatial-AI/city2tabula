# Changelog

Each release is listed with its breaking changes first. Releases before
v0.8.0 are described in their tag messages.

## Unreleased

Breaking changes:
- TABULA matching uses attached_neighbour_class as a ninth dimension,
  so re-extracting an existing database changes the TABULA type of
  some buildings (#13).
- TABULA matching ignores variant values TABULA leaves empty, measures
  distance over the dimensions both sides know, and normalises the
  integer codes without integer division. Re-extracting changes the
  TABULA type of many buildings (#174, #169).
- number_of_storeys counts the storeys below the eave with a 2.8 m
  floor-to-floor height (STOREY_HEIGHT) instead of dividing by the 2.5 m
  room height, plus one storey when the roof space reaches 2 m clear
  height. area_total_floor adds the attic floor area weighted as in
  WoFlV § 4, and TABULA matching uses the storeys below the eave. Storey
  counts, floor areas and TABULA types change on re-extraction (#175).
- TABULA matching compares TABULA's conditioned volume V_C with the
  building's eave height x footprint (min_volume) instead of its ridge
  height x footprint, since V_C leaves out an unheated attic (#176).

Features:
- _building carries full_storeys, attic_storey, attic_floor_area and
  storey_height; the storey correction trigger follows storey_height
  instead of room_height (#175).
- -extract-features marks buildings that share a wall with another
  building: has_attached_neighbour, attached_neighbour_id,
  total_attached_neighbour and attached_neighbour_class are filled
  instead of the fixed values used until now. Neighbours are listed by
  object_id, and GET /api/v1/buildings serves has_attached_neighbour,
  attached_neighbour_class and attached_neighbour_id. TABULA matching
  runs once after every batch instead of once per batch (#13).
- GET /api/v1/buildings serves each surface's length, width and height,
  and the OpenAPI Surface schema defines how every surface value is
  measured, with references.

Fixes:
- Eave height (min_height) and ridge height (max_height) come from the
  roof faces: the eave is the roof-area-weighted mean of the faces'
  lowest points, the ridge their highest point. min_height was the
  tallest wall, which is the ridge on buildings with gable walls, and
  max_height lay a roof rise above the roof, so storeys, volumes, floor
  area and TABULA types were inflated for pitched-roof buildings (#173).

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
