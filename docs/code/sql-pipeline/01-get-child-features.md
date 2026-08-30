# Script 01 — Get Child Features

**File:** `sql/scripts/main/01_get_child_feat.sql`  
**Reads from:** `{lod_schema}.feature`, `{lod_schema}.property`, `{lod_schema}.geometry_data`, `{lod_schema}.objectclass`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_child_feature`

---

## Purpose

A CityDB building solid is stored as a single feature, but it is *composed of* many smaller surface features: the roof faces, wall faces, and ground faces. This script finds those child surface features for each building by following the CityDB `boundary` property, then writes one row per surface feature per building into `_child_feature`.

---

## Background: how features are stored in CityDB

CityDB does not store a building and its surfaces in the same table row. Instead:

- A **building** is a row in `feature` with an `objectclass_id` in the range 900–999 (`901` Building, `902` BuildingPart).
- Its **surfaces** (WallSurface `709`, GroundSurface `710`, RoofSurface `712`) are separate `feature` rows.
- The `property` table carries the feature hierarchy. The feature that owns the solid holds one `boundary` property per surface, with `val_feature_id` pointing at the surface feature. It also holds one `lodNSolid` property whose `val_geometry_id` points at the solid geometry in `geometry_data`.
- Each surface feature holds one `lodNMultiSurface` property per LoD it is modelled at, with `val_geometry_id` pointing at that LoD's geometry.

For most datasets (DE, AT) the Building feature owns both the solid and the `boundary` rows. For 3DBAG (NL) the geometry sits on a child BuildingPart, which owns the solid and the `boundary` rows; the Building feature above it only carries thematic attributes. In both cases the feature holding `lodNSolid` also holds that building's `boundary` rows, so the script keys on that feature.

3DBAG also ships every building at LoD 1.2, 1.3 and 2.2 as separate surface features under the same BuildingPart. Filtering the surface geometry to `lodNMultiSurface` for the requested LoD keeps only that representation.

---

## Step-by-step walkthrough

### Step 1 — `buildings` CTE

```sql
WITH buildings AS (
  SELECT DISTINCT f.id AS building_feature_id, f.objectid AS building_object_id
  FROM {lod_schema}.feature f
  JOIN {lod_schema}.property p ON f.id = p.feature_id
    AND p.name = 'lod' || {lod_level} || 'Solid'
  WHERE f.objectclass_id BETWEEN 900 AND 999
    AND f.id NOT IN (
      SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature
    )
    AND f.id IN {building_ids}
)
```

Selects the feature that owns the solid for each building in the current batch:

1. **`p.name = 'lodNSolid'`** — the feature carrying the full building solid, which is also the feature carrying the `boundary` rows.
2. **`objectclass_id BETWEEN 900 AND 999`** — buildings and building parts only.
3. **`f.id NOT IN (...)`** — idempotency guard: skips buildings already in `_child_feature` so re-runs are safe.
4. **`f.id IN {building_ids}`** — restricts to the current batch.

### Step 2 — Main SELECT: follow `boundary`, then take the LoD's geometry

```sql
FROM buildings b
JOIN {lod_schema}.property boundary_link ON boundary_link.feature_id = b.building_feature_id
  AND boundary_link.name = 'boundary'
JOIN {lod_schema}.feature sf ON sf.id = boundary_link.val_feature_id
JOIN {lod_schema}.objectclass oc ON oc.id = sf.objectclass_id
JOIN {lod_schema}.property surface_geom ON surface_geom.feature_id = sf.id
  AND surface_geom.name = 'lod' || {lod_level} || 'MultiSurface'
JOIN {lod_schema}.geometry_data g ON g.id = surface_geom.val_geometry_id
WHERE sf.objectclass_id NOT BETWEEN 900 AND 999
  AND GeometryType(g.geometry) = 'MULTIPOLYGON'
```

- **`boundary_link`** — every surface feature attached to this building.
- **`surface_geom`** — the surface's geometry for the requested LoD. A surface modelled only at another LoD is dropped here.
- **`GeometryType = 'MULTIPOLYGON'`** — keeps polygon-based surfaces; script 02 explodes these into individual faces.

---

## Output columns

| Column | Description |
|--------|------------|
| `id` | Auto-generated UUID for this row |
| `lod` | LoD level (2 or 3) |
| `building_feature_id` | Feature id of the building (or BuildingPart) that owns the solid |
| `surface_feature_id` | Feature id of the surface |
| `building_object_id` / `surface_object_id` | Stable CityDB object ids, carried through all downstream tables |
| `objectclass_id` | Numeric type code (`709` WallSurface, `710` GroundSurface, `712` RoofSurface) |
| `classname` | Human-readable type name |
| `geom` | 3D MULTIPOLYGON geometry of the surface at this LoD |

---

## What comes next

Script 02 takes these MULTIPOLYGON geometries and explodes each one into individual POLYGON faces, because the normal and attribute calculations in script 03 operate on single faces.
