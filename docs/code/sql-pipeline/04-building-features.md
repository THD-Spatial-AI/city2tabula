# Script 04 — Building Features

**File:** `sql/scripts/main/04_calc_bld_feat.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_surface_raw`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building`

---

## Purpose

Scripts 01–03 produce one row per polygon face. This script collapses all those face-level rows into **one summary row per building**. It aggregates surface areas, counts faces, computes building height, and classifies shape complexity into the `_building` table.

---

## Step-by-step walkthrough

### Step 1 — `new_buildings` CTE

```sql
WITH new_buildings AS (
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_surface_raw
  WHERE building_feature_id IN {building_ids}
)
```

Unlike previous scripts, there is no "already processed" exclusion here — this CTE simply scopes the query to the current batch. The INSERT will naturally skip buildings already in the output table if `building_feature_id` is a unique key.

---

### Step 2 — `aggregated_surfaces` CTE

This CTE does most of the work, grouping all surface rows by building and computing summary values. The key computed columns are:

#### Surface areas by type

```sql
SUM(surface_area) FILTER (WHERE classname = 'GroundSurface') AS footprint_area,
SUM(surface_area) FILTER (WHERE classname = 'RoofSurface')   AS area_total_roof,
SUM(surface_area) FILTER (WHERE classname = 'WallSurface')   AS area_total_wall,
SUM(surface_area) FILTER (WHERE classname = 'GroundSurface') AS area_total_floor,
```

`FILTER (WHERE ...)` applies the `SUM` only to rows matching the condition. This avoids needing separate subqueries for each surface type.

Note that `area_total_floor` is initialised here to the same value as `footprint_area` (the raw GroundSurface area sum). Script 06 will overwrite it with `footprint_area × number_of_storeys` to represent total heated floor area across all floors.

#### Height

```sql
MAX(height) FILTER (WHERE classname = 'WallSurface') AS min_height,

MAX(height) FILTER (WHERE classname = 'WallSurface') +
COALESCE(MAX(height) FILTER (WHERE classname = 'RoofSurface'), 0) AS max_height,
```

Height is derived indirectly from the vertical spans of individual surface faces.

- **`min_height`** (eave height) — the maximum vertical span of any single wall face. This approximates the height to the eave (where the walls meet the roof), because a wall face typically spans the full height of the building's vertical portion.
- **`max_height`** (ridge height) — eave height plus the maximum vertical span of any single roof face. This approximates the height to the ridge (highest point of the roof).

The column names `min_height` / `max_height` refer to minimum and maximum height estimates of the building, not the smallest and largest face heights.

#### Footprint complexity

```sql
CASE
  WHEN ST_NPoints(ST_Boundary(ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'))) <= 4 THEN 0
  WHEN ST_NPoints(...)  BETWEEN 5 AND 10 THEN 1
  ELSE 2
END AS footprint_complexity,
```

The building footprint is reconstructed by unioning all GroundSurface polygons into one geometry and counting the vertices on its outer boundary:

| Vertex count | Code | Meaning |
|---|---|---|
| ≤ 4 | 0 | Simple (rectangle or triangle) |
| 5–10 | 1 | Regular (L-shape, U-shape) |
| > 10 | 2 | Complex (many-sided, irregular) |

#### Roof complexity

```sql
CASE
  WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') = 1 THEN 0
  WHEN COUNT(*) FILTER (WHERE classname = 'RoofSurface') BETWEEN 2 AND 4 THEN 1
  ELSE 2
END AS roof_complexity,
```

Measured by the number of distinct RoofSurface polygon faces:

| Face count | Code | Meaning |
|---|---|---|
| 1 | 0 | Simple (flat or single-pitch) |
| 2–4 | 1 | Regular (gable, hip) |
| > 4 | 2 | Complex (multi-faceted, mansard) |

#### Storey count (initial estimate)

```sql
CASE
  WHEN MAX(height) FILTER (WHERE classname = 'WallSurface') > 0 AND 2.5 > 0
  THEN MAX(height) FILTER (WHERE classname = 'WallSurface') / 2.5
  ELSE 1
END AS number_of_storeys,
```

A first rough estimate: wall height divided by a default room height of 2.5 m. This is refined in script 06 using the same formula but with the properly stored values, which allows the room height to be overridden per dataset.

#### Building footprint geometry

```sql
ST_Transform(ST_Force2D(ST_Centroid(
    ST_Union(geom) FILTER (WHERE classname = 'GroundSurface')
)), {srid}) AS building_centroid_geom,

ST_Transform(
    ST_Union(geom) FILTER (WHERE classname = 'GroundSurface'),
{srid}) AS building_footprint_geom
```

The merged GroundSurface geometry is re-projected to the target CRS (`{srid}`). The centroid is the geometric centre of the merged footprint, used for mapping.

---

## Placeholder columns

Several columns are set to placeholder values that are not yet available at this stage:

| Column | Initial value | Updated by |
|--------|--------------|-----------|
| `construction_year` | 0 | External data (not automated) |
| `heating_demand` | 0.0 | External energy model |
| `has_attached_neighbour` | `FALSE` | Not yet implemented |
| `surface_count_floor` | 0 | Not computed (ground is counted differently) |
| `area_total_floor` | `= footprint_area` | Script 06 (overwritten) |
| `number_of_storeys` | `wall_height / 2.5` | Script 06 (refined) |

---

## Output columns (key)

| Column | Description |
|--------|------------|
| `footprint_area` | Sum of all GroundSurface face areas (sqm) |
| `footprint_complexity` | 0 = simple, 1 = regular, 2 = complex |
| `roof_complexity` | 0 = simple, 1 = regular, 2 = complex |
| `area_total_roof` | Sum of all RoofSurface face areas (sqm) |
| `area_total_wall` | Sum of all WallSurface face areas (sqm) |
| `area_total_floor` | Initially = footprint_area; overwritten in script 06 |
| `min_height` | Eave height — max wall face span (m) |
| `max_height` | Ridge height — eave + max roof face span (m) |
| `number_of_storeys` | Wall height / 2.5; refined in script 06 |
| `building_centroid_geom` | 2D centroid of merged footprint |
| `building_footprint_geom` | Merged 2D footprint geometry |

---

## What comes next

Script 05 adds volume estimates (height × footprint area). Script 06 then refines storey count and overwrites the floor area.
