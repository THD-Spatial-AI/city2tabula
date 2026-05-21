# Script 02 — Dump Geometry

**File:** `sql/scripts/main/02_dump_child_feat_geom.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_child_feature`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_child_feature_geom_dump`

---

## Purpose

Script 01 stored one row per surface feature, and each geometry is a `MULTIPOLYGON` — a single object that groups several polygon faces together. A building's roof, for example, might be stored as one `MULTIPOLYGON` containing four triangular faces.

For tilt, azimuth, and area calculations (script 03) to work correctly, each face must be its own row. This script performs that **explosion**: one `MULTIPOLYGON` row becomes many individual `POLYGON` rows, one per face.

---

## Background: MULTIPOLYGON vs POLYGON

A `POLYGON` is a single closed ring of vertices representing one flat face. A `MULTIPOLYGON` is a collection of polygons bundled into one geometry object. Think of it like the difference between a single page (polygon) and a book (multipolygon):

- A gable roof might be stored as one `MULTIPOLYGON` containing two triangular slope faces.
- After this script, those two faces become two separate `POLYGON` rows.

PostGIS provides `ST_Dump()` to perform this split. It returns a set of rows — one per polygon component — so a single input row yields as many output rows as there are polygon faces.

---

## Step-by-step walkthrough

### Step 1 — `new_buildings` CTE

```sql
WITH new_buildings AS (
  SELECT DISTINCT building_feature_id
  FROM {city2tabula_schema}.{lod_schema}_child_feature
  WHERE building_feature_id IN {building_ids}
    AND building_feature_id NOT IN (
      SELECT DISTINCT building_feature_id
      FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
    )
)
```

Selects building IDs from the current batch that do not yet have rows in the output table. This is the idempotency guard — if a building's geometry was already dumped in a previous run, it is skipped entirely.

---

### Step 2 — `dumped` CTE

```sql
dumped AS (
  SELECT
    c.id AS child_row_id,
    c.building_feature_id,
    c.surface_feature_id,
    c.objectclass_id,
    c.classname,
    (ST_Dump(c.geom)).geom AS geom
  FROM {city2tabula_schema}.{lod_schema}_child_feature c
  INNER JOIN new_buildings nb ON c.building_feature_id = nb.building_feature_id
)
```

`ST_Dump(geom)` is a set-returning function: each call produces one row for every polygon component inside the multipolygon. The `.geom` field extracts just the geometry (there is also a `.path` field for array-position tracking, not used here).

**Example:** a surface feature with a 4-face MULTIPOLYGON produces 4 rows in `dumped`, all sharing the same `child_row_id`, `building_feature_id`, and `classname`.

---

### Step 3 — INSERT

```sql
SELECT
    gen_random_uuid() AS id,
    ...
    ST_CoordDim(geom) AS coord_dim,
    (ST_ZMin(geom) IS NOT NULL) AS has_z,
    geom::geometry(POLYGONZ) AS geom
FROM dumped
```

Two diagnostic columns are added:

- **`coord_dim`** — the number of coordinate dimensions (2 for XY, 3 for XYZ). Detected via `ST_CoordDim`. Script 03 needs Z coordinates to compute normals; this flags rows that might be missing them.
- **`has_z`** — `true` if `ST_ZMin` returns a non-null value, confirming 3D data is present.

The geometry is cast to `POLYGONZ` (an explicit PostGIS geometry type with a Z coordinate) to lock in the 3D type. If the cast fails, the row had no Z data and would have caused silent errors downstream.

---

## Output columns

| Column | Description |
|--------|------------|
| `id` | Auto-generated UUID for this row |
| `child_row_id` | UUID of the parent row in `_child_feature` |
| `building_feature_id` | ID of the parent building |
| `surface_feature_id` | ID of the surface feature |
| `objectclass_id` | Numeric type code |
| `classname` | Surface type (`RoofSurface`, `WallSurface`, `GroundSurface`) |
| `coord_dim` | Coordinate dimensionality (should be 3) |
| `has_z` | Whether Z coordinates are present |
| `geom` | Single POLYGONZ face |

---

## What comes next

Script 03 reads these individual polygon faces and computes a surface normal for each one, from which tilt, azimuth, area, and height are derived.
