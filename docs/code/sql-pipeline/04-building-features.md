---
audience: developer
---

# Script 04: Building Features

**File:** `sql/scripts/main/04_calc_bld_feat.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_surface_raw`, `{lod_schema}.feature`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building_part`, `{city2tabula_schema}.{lod_schema}_building`

---

## Purpose

Scripts 01–03 produce one row per polygon face. This script collapses those face-level rows in two stages: **one row per solid** in `_building_part`, then **one summary row per building** in `_building`. A building's solids are the Building itself and/or its BuildingParts (see [Buildings and BuildingParts](index.md#buildings-and-buildingparts)).

Every area uses the face's **exposed area**, `surface_area − area_internal`. Faces that lie between two solids of the same building (script 03) therefore do not count as envelope.

---

## Stage 1: `_building_part`

One row per `owner_feature_id`:

| Column | Value |
|--------|-------|
| `owner_feature_id` / `owner_object_id` | The solid owner's feature id and object id |
| `building_feature_id` | The Building it belongs to |
| `footprint_area` | Sum of exposed GroundSurface area (sqm) |
| `min_height` | Eave height: the maximum vertical span of any WallSurface face (m) |
| `max_height` | Ridge height: eave height plus the maximum vertical span of any RoofSurface face (m) |

The column names `min_height` / `max_height` refer to minimum and maximum height estimates of the solid, not the smallest and largest face heights. The insert uses `ON CONFLICT DO NOTHING`, so a retried task does not duplicate rows.

---

## Stage 2: `_building`

A building with no WallSurface or RoofSurface face gets no `_building` row, so it is not classified, linked or served, and script 08 writes no `_surface` rows for it. Its faces stay in `_surface_raw`.

### Heights: footprint-weighted over the solids

```sql
SUM(min_height * footprint_area) / SUM(footprint_area)   -- per building, from _building_part
```

A building's `min_height` and `max_height` are the means of its solids' heights, each weighted by the solid's footprint. Script 05's `height × footprint_area` and script 06's `footprint_area × number_of_storeys` then equal the sums over the solids, so a tower does not lend its height to the podium beside it. The per-solid heights stay in `_building_part`. A building whose solids have no ground area falls back to its tallest solid.

The initial `number_of_storeys` is `min_height` divided by the default room height of 2.5 m (1 when `min_height` is 0 or missing). Script 06 refines it.

### Surface areas by type

```sql
SUM(exposed_area) FILTER (WHERE classname = 'GroundSurface') AS footprint_area,
SUM(exposed_area) FILTER (WHERE classname = 'RoofSurface')   AS area_total_roof,
SUM(exposed_area) FILTER (WHERE classname = 'WallSurface')   AS area_total_wall,
SUM(exposed_area) FILTER (WHERE classname = 'GroundSurface') AS area_total_floor,
```

`area_total_floor` starts as the GroundSurface sum; script 06 overwrites it with the total heated floor area.

### Surface counts

`surface_count_roof`, `surface_count_wall` and `surface_count_floor` count the rows script 08 serves: one per face, one per exposed piece of a partly internal face, none for a fully internal face.

### Footprint complexity

The building footprint is the union of the GroundSurface polygons of all its solids, so the edges between touching solids dissolve. Its outer-boundary vertex count gives the code:

| Vertex count | Code | Meaning |
|---|---|---|
| ≤ 4 | 0 | Simple (rectangle or triangle) |
| 5–10 | 1 | Regular (L-shape, U-shape) |
| > 10 | 2 | Complex (many-sided, irregular) |

### Roof complexity

Measured by the number of exposed RoofSurface polygons:

| Count | Code | Meaning |
|---|---|---|
| 1 | 0 | Simple (flat or single-pitch) |
| 2–4 | 1 | Regular (gable, hip) |
| > 4 | 2 | Complex (multi-faceted, mansard) |

### Building footprint geometry

The merged GroundSurface geometry is re-projected to the target CRS (`{srid}`) as `building_footprint_geom`. Its 2D centroid is `building_centroid_geom`, used for mapping.

---

## Placeholder columns

| Column | Initial value | Updated by |
|--------|--------------|-----------|
| `construction_year` | 0 | External data (not automated) |
| `has_attached_neighbour` | `FALSE` | Not yet implemented |
| `area_total_floor` | Exposed GroundSurface sum | Script 06 (overwritten) |
| `number_of_storeys` | `min_height` / 2.5 | Script 06 (refined) |

---

## Output columns (key)

| Column | Description |
|--------|------------|
| `object_id` | The Building's object id |
| `footprint_area` | Sum of exposed GroundSurface area over all solids (sqm) |
| `footprint_complexity` | 0 = simple, 1 = regular, 2 = complex |
| `roof_complexity` | 0 = simple, 1 = regular, 2 = complex |
| `area_total_roof` | Sum of exposed RoofSurface area (sqm) |
| `area_total_wall` | Sum of exposed WallSurface area (sqm) |
| `area_total_floor` | Initially the GroundSurface sum; overwritten in script 06 |
| `min_height` | Footprint-weighted mean eave height of the solids (m) |
| `max_height` | Footprint-weighted mean ridge height of the solids (m) |
| `number_of_storeys` | `min_height` / 2.5; refined in script 06 |
| `building_centroid_geom` | 2D centroid of the merged footprint |
| `building_footprint_geom` | Merged 2D footprint geometry |

---

## What comes next

Script 05 adds volume estimates (height × footprint area). Script 06 then refines the storey count and overwrites the floor area.
