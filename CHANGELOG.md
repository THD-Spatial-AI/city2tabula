# Changelog

Each release is listed with its breaking changes first. Releases before
v0.8.0 are described in their tag messages.

## Unreleased

Features:
- A bbox import skips citydb-tool for a dataset when an earlier import
  from the same source files already covered the bbox, so repeating a
  run over an imported area no longer re-reads the source files. Each
  LOD schema gains a c2t_import_extent table recording the imported
  bboxes per dataset. Adding, replacing or removing a source file
  invalidates the dataset's records. Imports without a bbox, or with
  IMPORT_LIMIT set, are neither skipped nor recorded.
- GET /api/v1/runs/{id} and POST /api/v1/runs report the pipeline step
  a run is in: stage (import, extract_features or link_buildings),
  stage_label, stage_index and stage_count, plus created_at,
  started_at and updated_at. The stage fields and started_at are
  absent while a run is pending.
- Logging is structured: every line carries time, level, the source
  file and line, and the message, as key=value text or, with
  LOG_FORMAT=json, one JSON object per line. The existing log calls
  keep working through the same handler. Debug lines are no longer
  written to the log file at INFO level, and error lines go to stdout
  with the rest instead of stderr.
- The HTTP server logs one line per request (method, path, query,
  status, duration) and returns an X-Request-ID header, reusing a
  valid id the caller sent. Health checks log at DEBUG.

Fixes:
- An on-request run opens two database pools instead of three, and
  the per-connection PostGIS extension checks log at DEBUG instead of
  INFO, so the server log no longer repeats them for every run.
- The run status read a run record the pipeline goroutine was still
  writing, without a lock. The server now returns a copy.

## v0.10.1 (2026-10-14)

Fixes:
- A successful feature extraction logs "Feature extraction complete"
  with its building count and duration, and an on-request run logs its
  completion with the run id, country, linked building count and
  duration. Before, the log of a successful run stopped at "Extracting
  Features", which read as a hung run. The two on-request failure paths
  that set a run failed without logging now log the error too.
- The city2tabula and city2tabula-db services in
  environment/http/docker-compose.yml restart unless stopped, so they
  come back after a Docker or host restart. city2tabula-cli, which is
  run on demand, is unchanged.

## v0.10.0 (2026-10-13)

Breaking changes:
- The surface fields length and width are replaced by
  horizontal_extent and slope_extent, on lod2_surface_raw,
  lod2_surface and the surfaces of GET /api/v1/buildings. length and
  width were the long and short side of the smallest rectangle around
  the face, so neither had a fixed direction. horizontal_extent is the
  face's extent along the horizontal in its plane (a wall's run, a
  roof's eave length), slope_extent its extent up the fall line (a
  wall's height, a roof's slope length). Faces within about 10 degrees
  of horizontal, such as ground faces and flat roofs, have neither.
  Existing databases need the city2tabula schema recreated (#188).

## v0.9.0 (2026-10-09)

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
- Walls shared with attached neighbours (party walls) are left out of
  area_total_wall and surface_count_wall and counted in the new
  area_party_wall; TABULA matching sees the envelope walls only. Wall
  areas and TABULA types change for attached buildings on
  re-extraction. The served surfaces are rebuilt after every extraction,
  and neighbour_building_id is replaced by neighbour_object_id (#14).

Features:
- The server's compose file passes PYLOVO_FDW_HOST, _PORT, _DBNAME,
  _USER and _PASSWORD through to the container, so on-request runs can
  link buildings to PyLovo. Left unset, linking stays a no-op.
- Party walls: the part of each wall that lies against an attached
  neighbour's wall is served as its own surface piece with
  is_party_wall and neighbour_object_id, on GET /api/v1/buildings and
  GET /api/v1/geometry, so a viewer can highlight it (#14).
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
- is_valid tests each face in its own plane instead of its plan view,
  where every vertical wall failed, and is_planar allows 0.01 m off the
  face's mid-plane instead of none (#172).
- ROOM_HEIGHT is no longer read. It had no effect: room_height is
  fixed at TABULA's 2.5 m reference room height.
- Connections run with PostgreSQL's JIT compiler off. Compiling the
  extraction queries took longer than running them: script 03 spent
  6.3 of 7.5 s per 500-building batch compiling on PostgreSQL 17.

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
