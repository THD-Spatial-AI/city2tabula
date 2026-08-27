# API reference

The interactive reference lives in its own standalone page, [`openapi/index.html`](openapi/index.html), so it can be opened directly without running `mkdocs serve`. It renders [`openapi/openapi.yaml`](openapi/openapi.yaml); download that file to generate a client or import it into Postman.

!!! warning "No authentication"
    This API has none of its own and is not behind a reverse proxy. It's meant for trusted internal callers on the same network — do not expose it directly to the public internet as-is.

!!! note "Try it out needs a proxy or same-origin setup"
    The server sends no CORS headers, so Swagger UI's **Try it out** will fail against a real running instance from this docs page. Use `curl` (examples below) for hands-on testing until that's addressed.

## Workflow

| Step | Endpoint | Purpose |
|---|---|---|
| 1 | `GET /api/v1/coverage` | Cheap check: does this bbox already have extracted data? |
| 2 | `POST /api/v1/runs` | If not, trigger extraction. Returns a `run_id` immediately. |
| 3 | `GET /api/v1/runs/{id}` | Poll until `status` is `completed`, `no_data`, or `failed`. |
| 4 | `GET /api/v1/buildings` | Building + surface attributes, once data exists. |
| 5 | `GET /api/v1/geometry` | Footprint geometry, only if something needs to render it. |

`buildings` and `geometry` are separate endpoints on purpose: a calculation consumer doesn't need geometry, only a visualization consumer does, so it isn't fetched unless actually needed.

## Health check

```bash
curl http://localhost:5000/api/v1/health
```

Returns `{"status": "ok"}` if the process is up. Not authoritative for any one country — it doesn't touch a database connection.

## Checking coverage

```bash
curl "http://localhost:5000/api/v1/coverage?country=germany&xmin=8.79&ymin=53.14&xmax=8.82&ymax=53.16"
```

Returns `{"count": N}` — the number of already PyLovo-linked buildings in that bbox. `count: 0` means trigger a run before reading buildings.

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as City2TABULA API
    participant DB as Country database

    C->>S: GET /api/v1/coverage?country&bbox
    S->>DB: count building_link rows intersecting bbox
    DB-->>S: count
    S-->>C: 200 {count}
```

## Triggering a run

```bash
curl -X POST http://localhost:5000/api/v1/runs \
  -H 'Content-Type: application/json' \
  -d '{"country":"germany","xmin":8.79,"ymin":53.14,"xmax":8.82,"ymax":53.16}'
```

The pipeline (import, feature extraction, PyLovo linking) runs in the background; the request returns immediately with a `run_id` to poll.

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as City2TABULA API
    participant P as Pipeline (import, extract, link)
    participant DB as Country database

    C->>S: POST /api/v1/runs {country, bbox, bbox_mode}
    S->>S: validate request, create Run (status: pending)
    S-->>C: 202 Accepted {run_id, status: pending}
    S->>P: executeRun(run_id) — background goroutine
    P->>DB: import 3D data scoped to bbox (create or incremental)
    P->>DB: extract features (scripts 01-08)
    P->>DB: link PyLovo buildings (IoU join into building_link)
    P->>DB: count building_link rows in bbox
    alt buildings found
        P->>S: set Run status: completed
    else no source data in bbox
        P->>S: set Run status: no_data
    else pipeline error
        P->>S: set Run status: failed, error
    end
```

## Polling a run

```bash
curl http://localhost:5000/api/v1/runs/<run_id>
```

`no_data` means the pipeline ran successfully but found no source data for that bbox — not an error. `failed` means a real error; check `error` in the response.

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as City2TABULA API

    loop until status is completed, no_data, or failed
        C->>S: GET /api/v1/runs/{id}
        S-->>C: 200 {run_id, status, error}
        Note over C: wait, then poll again
    end
```

## Reading buildings

By bbox, independent of any PyLovo link:

```bash
curl "http://localhost:5000/api/v1/buildings?country=germany&xmin=8.79&ymin=53.14&xmax=8.82&ymax=53.16"
```

By PyLovo OSM id, once a link exists:

```bash
curl "http://localhost:5000/api/v1/buildings?country=germany&osm_ids=123456,789012"
```

Each building's `surfaces` array carries per-element area/azimuth/tilt.

!!! warning "Tilt convention"
    `tilt` here is 0=vertical wall, 90=flat roof — the opposite of the common building-energy convention (0=horizontal roof, 90=vertical wall). Invert before feeding it to a consumer that expects that convention.

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as City2TABULA API
    participant DB as Country database

    alt osm_ids given (PyLovo-linked buildings)
        C->>S: GET /api/v1/buildings?country&osm_ids=a,b,c
        S->>DB: join building_link + lod2_building on osm_id
    else bbox given (every building, linked or not)
        C->>S: GET /api/v1/buildings?country&xmin&ymin&xmax&ymax
        S->>DB: select lod2_building intersecting bbox
    end
    DB-->>S: building rows
    S->>DB: batch-fetch surfaces for the returned object_ids
    DB-->>S: surface rows
    S-->>C: 200 [{object_id, osm_id, match_type, ..., surfaces: [...]}]
```

`osm_ids` takes precedence if both are present in the query string.

## Reading geometry

```bash
curl "http://localhost:5000/api/v1/geometry?country=germany&object_ids=DEHB01AL3AU0004T"
```

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as City2TABULA API
    participant DB as Country database

    C->>S: GET /api/v1/geometry?country&object_ids=a,b,c
    S->>DB: select footprint geometry for object_ids
    DB-->>S: geometry rows (GeoJSON)
    S-->>C: 200 [{object_id, footprint_geojson}]
```
