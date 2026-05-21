# Script 01 — Get Child Features

**File:** `sql/scripts/main/01_get_child_feat.sql`  
**Reads from:** `{lod_schema}.feature`, `{lod_schema}.geometry_data`, `{lod_schema}.property`, `{lod_schema}.objectclass`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_child_feature`

---

## Purpose

A CityDB building solid is stored as a single feature, but it is *composed of* many smaller surface features — the roof faces, wall faces, and ground faces. This script finds those child surface features for each building by asking: *which features in the database are geometrically inside or touching this building's solid?*

The result is one row per surface feature per building, stored in `_child_feature`.

---

## Background: how features are stored in CityDB

CityDB does not store a building and its surfaces in the same table row. Instead:

- A **building** is a row in `feature` with an `objectclass_id` in the range 900–999.
- Its **surfaces** (RoofSurface, WallSurface, etc.) are separate rows in the same `feature` table, each with their own `objectclass_id` outside that range.
- Geometries live in `geometry_data`, linked via `feature_id`.
- The building solid is identified by a `property` row with `name = 'lod2Solid'` (or `lod3Solid`).

There is no direct foreign key from surface rows to their parent building. Instead, this script uses **3D spatial intersection** to discover the relationship.

---

## Step-by-step walkthrough

### Step 1 — `buildings` CTE

```sql
WITH buildings AS (
  SELECT f.id AS building_feature_id, g.geometry AS building_geom
  FROM {lod_schema}.feature f
  JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
  JOIN {lod_schema}.property p ON f.id = p.feature_id
    AND p.name = 'lod' || {lod_level} || 'Solid'
  WHERE objectclass_id BETWEEN 900 AND 999
    AND f.id NOT IN (
      SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature
    )
    AND f.id IN {building_ids}
)
```

This CTE selects the building features for the current batch. Three filters apply:

1. **`objectclass_id BETWEEN 900 AND 999`** — keeps only rows that represent buildings (not surfaces, furniture, trees, etc.).
2. **`p.name = 'lod2Solid'`** — joins to the property that holds the full building solid geometry, not just a wall or footprint face.
3. **`f.id NOT IN (...)`** — idempotency guard: skips any building that already has rows in `_child_feature` so re-runs are safe.

The output is a small temporary result set: one row per unprocessed building containing the building's ID and its 3D solid geometry.

---

### Step 2 — Main SELECT with `ST_3DIntersects`

```sql
SELECT ...
FROM {lod_schema}.feature f
JOIN {lod_schema}.objectclass oc ON f.objectclass_id = oc.id
JOIN {lod_schema}.geometry_data g ON f.id = g.feature_id
JOIN buildings b ON ST_3DIntersects(g.geometry, b.building_geom)
WHERE f.objectclass_id NOT BETWEEN 900 AND 999
  AND f.id != b.building_feature_id
  AND GeometryType(g.geometry) IN ('MULTIPOLYGON')
```

This is the heart of the script. For every feature in the database (all rows), it asks: *does this feature's geometry intersect the building's 3D solid?* If yes, and if the feature is not itself a building (objectclass outside 900–999) and is not the building we're querying for, it's a child surface.

**`ST_3DIntersects`** performs a 3D spatial intersection test. A wall surface that shares a face with the building solid will return true. Features on the other side of the city return false and are discarded.

**`GeometryType IN ('MULTIPOLYGON')`** keeps only polygon-based surface features. Points, lines, and other geometry types are excluded because they cannot represent surface areas.

The result is inserted into `_child_feature` with a generated UUID, the LoD level, the building ID, the surface feature ID, its object class, class name, and geometry.

---

## Output columns

| Column | Description |
|--------|------------|
| `id` | Auto-generated UUID for this row |
| `lod` | LoD level (2 or 3) |
| `building_feature_id` | ID of the parent building |
| `surface_feature_id` | ID of the surface feature |
| `objectclass_id` | Numeric type code (e.g. 709 = RoofSurface) |
| `classname` | Human-readable type name (e.g. `RoofSurface`) |
| `geom` | 3D MULTIPOLYGON geometry of the surface |

---

## What comes next

Script 02 takes these MULTIPOLYGON geometries and explodes each one into individual POLYGON faces, because normal and attribute calculations in script 03 operate on single faces.
