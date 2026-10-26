# Changelog

Each release is listed with its breaking changes first. Releases before
v0.8.0 are described in their tag messages.

## v$V ($DATE)

Features:
- A bbox import (POST /api/v1/runs, -import-data -bbox) passes
  citydb-tool only the files whose declared extent intersects the bbox:
  a CityGML boundedBy envelope, or a CityJSON metadata
  geographicalExtent. It records the bbox against those files alone,
  so a region import reads only that region's tiles, and adding tiles
  elsewhere in the dataset folder leaves earlier regions' records valid.
  A file without a readable extent or a recognised CRS is always
  passed. On all 18,297 Bavarian tiles in one folder, a region of about
  2 by 1 km imports 4 files in 9 s; a repeat run takes about 1 s.

## v0.14.0 (2026-10-26)

Features:
- -import-data and -create-db record a whole-folder import by its
  files' fingerprint, as bbox imports already were, so a later import
  skips citydb-tool for every dataset folder whose files are unchanged.
  A scheduled `c2t -import-data` is then cheap when nothing is new. A
  bbox run after a whole-folder import of unchanged files is skipped
  too. docs/deployment/adding-data.md covers the scheduled import, the
  staging of new files and the attribution check.
- c2t -check-data <dir> checks a staged dataset folder for COUNTRY
  without importing it or connecting to a database: attribution.json,
  the gml/ and cityjson/ folders, a dataset_id no published folder uses
  with another attribution, and each model file's format and declared
  CRS, which must be the SRID the import uses. It lists every problem
  and exits with status 1, or prints "ready to publish".

## v0.13.0 (2026-10-25)

Features:
- GET /api/v1/surfaces returns the roof and wall surfaces of every
  building in a WGS84 bbox, for rendering: per building its object_id,
  its OSM ways and each surface's id (the same as /geometry's), type
  and GeoJSON Polygon in EPSG:4326 with height, to 7 decimals. Pages
  hold whole buildings up to about 4 MB, in object_id order; pass
  next_cursor back until it is null. /geometry still serves surfaces
  for one building at a time.

## v0.12.0 (2026-10-23)

Features:
- GET /api/v1/buildings returns use_type and residential next to
  function_code in all three modes. use_type maps the code to one of
  residential, office, retail, hospitality, education, health, public,
  commercial, industry, agriculture, ancillary or other; residential is
  true when the building contains dwellings, including mixed-use
  buildings. Germany is mapped from the AdV code list
  BuildingFunctionTypeAdV.xml; both fields are null where the code is
  missing, unmapped or states that the use is unknown. Existing
  databases return them without re-extraction.
- use_type and residential for the Netherlands, from BAG. Building
  attributes the 3D data lacks are staged under
  data/attributes/<country>/<dataset>/ as attribution.json plus
  building_use.csv (object_id, use_code, floor_area: one row per use of
  a building). -create-db and -import-data replace each dataset's rows
  in city2tabula.building_use and sync its credit. Where a building's
  function code gives no use, GET /api/v1/buildings takes the use
  covering the most floor area among its rows, residential when any row
  is a dwelling, and credits the dataset. A BAG building without units
  (a shed, garage or barn) has one row with use code "geen
  verblijfsobject" and is ancillary; a building without rows stays
  null. The server needs data/attributes mounted next to data/lod2.

Fixes:
- -link-pylovo, and the link stage of an on-request run, no longer
  stalls on a database holding regions far apart. Batching built a
  grid over the box around all buildings and tested every building
  against every cell: for Deggendorf and Koeln in one database that
  took 29 minutes before linking began. Each building's cell is now
  computed from its coordinates; the batches are unchanged.
- Feature extraction works on PostgreSQL 18. There every batch holding
  a flat roof or ground surface failed with division by zero in
  surface_dimensions, so no building was extracted. PostgreSQL 16 and
  17 were not affected. A database created by an earlier release gets
  the fixed function on its next -reset-city2tabula.

## v0.11.4 (2026-10-20)

Features:
- GET /api/v1/buildings takes object_ids, a third mode next to osm_ids
  and the bbox: it returns the named 3D buildings, linked to PyLovo or
  not, each once and with the same fields as the other modes. A linked
  building carries its ways and match_type 1, one without an OSM match
  match_type 2, one not yet linked match_type 0. When several modes are
  given, osm_ids wins, then object_ids, then the bbox.
- GET /api/v1/geometry takes srid=4326, which returns the footprint and
  any surfaces reprojected to WGS84 longitude and latitude, with no crs
  member; surfaces keep their height. Without it the geometry stays in
  the dataset's storage CRS. Any other srid value is a 400.

## v0.11.3 (2026-10-19)

Features:
- GET /api/v1/buildings returns each building's function_code, the
  CityGML function code of its source Building (in Germany the ALKIS
  Gebäudefunktion, for example 31001_1000), in both the osm_ids and the
  bbox mode. It is read from the stored source data when the response
  is built, so existing databases return it without re-extraction, and
  it is null where the dataset carries none (the NL, AT and CZ demo
  data) or where the database holds no source tables.

## v0.11.2 (2026-10-19)

Fixes:
- Every OSM way that covers a 3D building is linked, not only the best
  one (#203). Where OSM maps one building as several ways, the others
  had no building_link row, so a caller asking for them got no 3D data.
  The best way keeps match_type 1; each further way with overlap 0.5 or
  more gets a row with the new match_type 4 and the same object_id. In
  the German demo data this links 63 further ways on 49 buildings, and
  no building's best way changes. GET /api/v1/buildings?osm_ids=
  returns each building once however many of its ways are asked for,
  with a new osm_ids field listing all its ways, best first. A
  building_link table created by an earlier release is updated to one
  row per way on the next link run; existing links are filled in with
  -link-pylovo -relink.

## v0.11.1 (2026-10-18)

Fixes:
- A building's TABULA label no longer changes when other buildings are
  extracted (#200). The match scaled each dimension over all buildings
  in the database together with the variants, so extracting a new area
  could relabel buildings already stored. Volume, footprint, storeys
  and the roof, wall and floor areas are now scaled over the country's
  TABULA variants, and footprint complexity, roof complexity and
  attached neighbours over their fixed code range 0 to 2. Labels shift
  once, on the first relabel after upgrading: in the four demo
  databases 30 to 73 percent of labels change, mostly in the
  construction-period band, while 85 to 98 percent keep their building
  type. Existing databases keep the old scale in the correction trigger
  `<lod>_recalc_variant_match` until the city2tabula schema is
  recreated.

## v0.11.0 (2026-10-16)

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
- On-request run lines carry run_id, country, bbox and bbox_mode: run
  started (with the time spent waiting for an overlapping run), one
  line per stage, and run completed, run found no source data or run
  failed, the last naming the stage and the error. A run accepted line
  links the request id to the run id.

Fixes:
- A surface's id stays the same when the surface table is rebuilt.
  Every extraction rebuilt it and gave every surface in the country a
  new random UUID, so an id stored by a client stopped matching after
  any later run. The id is now an md5-derived UUID of the source
  surface's object id, the face's position in it and the exterior or
  party-wall piece. In an existing database the ids change once, on
  the first run after upgrading, and are stable from then on.
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
