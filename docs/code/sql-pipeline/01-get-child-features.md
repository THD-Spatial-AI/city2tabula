---
audience: developer
---

# Script 01: Get Child Features

**File:** `sql/scripts/main/01_get_child_feat.sql`  
**Reads from:** `{lod_schema}.feature`, `{lod_schema}.property`, `{lod_schema}.geometry_data`, `{lod_schema}.objectclass`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_child_feature`

---

## Purpose

A CityDB building is stored as a Building feature, one or more solids, and many smaller surface features: the roof faces, wall faces, and ground faces. This script collects the surface features of every solid that belongs to a Building, following the CityDB `buildingPart` and `boundary` properties, and writes one row per surface feature into `_child_feature`, keyed by the Building.

---

## Background: how features are stored in CityDB

CityDB does not store a building and its surfaces in the same table row. Instead:

- A **Building** is a row in `feature` with `objectclass_id` `901`. A **BuildingPart** (`902`) is a separate row, linked from its Building (or from another part) by a `buildingPart` property whose `val_feature_id` points at the part.
- Its **surfaces** (WallSurface `709`, GroundSurface `710`, RoofSurface `712`) are separate `feature` rows.
- A feature that owns a solid holds one `lodNSolid` property and one `boundary` property per surface, with `val_feature_id` pointing at the surface feature.
- Each surface feature holds one `lodNMultiSurface` property per LoD it is modelled at, with `val_geometry_id` pointing at that LoD's geometry.

Which features own solids depends on the source. See [Buildings and BuildingParts](index.md#buildings-and-buildingparts) for the patterns. The script takes every solid owner under the Building, so the same query covers all of them.

3DBAG also ships every building at LoD 1.2, 1.3 and 2.2 as separate surface features under the same BuildingPart. Filtering the surface geometry to `lodNMultiSurface` for the requested LoD keeps only that representation.

BuildingInstallation features (`905`: dormers, chimneys, balconies) are linked by a `buildingInstallation` property that the script does not follow. Their surfaces sit on an envelope the solids already close, so counting them would count that envelope twice.

---

## Step-by-step walkthrough

### Step 1: `members` CTE (recursive)

```sql
WITH RECURSIVE members AS (
  SELECT f.id AS building_feature_id, f.objectid AS building_object_id, f.id AS member_id
  FROM {lod_schema}.feature f
  WHERE f.objectclass_id = 901
    AND f.id IN {building_ids}
    AND f.id NOT IN (SELECT building_feature_id FROM {city2tabula_schema}.{lod_schema}_child_feature)
  UNION ALL
  SELECT m.building_feature_id, m.building_object_id, p.val_feature_id
  FROM members m
  JOIN {lod_schema}.property p ON p.feature_id = m.member_id AND p.name = 'buildingPart'
)
```

Lists each Building in the batch together with every BuildingPart below it, at any depth. The `NOT IN` guard skips Buildings already in `_child_feature`, so re-runs are safe.

### Step 2: `owners` CTE

Keeps the members that hold an `lodNSolid` property for the requested LoD. Each row carries the Building's `building_feature_id` and `building_object_id`, and the solid owner's id as `owner_feature_id`.

### Step 3: Main SELECT: follow `boundary`, then take the LoD's geometry

```sql
FROM owners o
JOIN {lod_schema}.property boundary_link ON boundary_link.feature_id = o.owner_feature_id
  AND boundary_link.name = 'boundary'
JOIN {lod_schema}.feature sf ON sf.id = boundary_link.val_feature_id
JOIN {lod_schema}.objectclass oc ON oc.id = sf.objectclass_id
JOIN {lod_schema}.property surface_geom ON surface_geom.feature_id = sf.id
  AND surface_geom.name = 'lod' || {lod_level} || 'MultiSurface'
JOIN {lod_schema}.geometry_data g ON g.id = surface_geom.val_geometry_id
WHERE sf.objectclass_id NOT BETWEEN 900 AND 999
  AND GeometryType(g.geometry) = 'MULTIPOLYGON'
```

- **`boundary_link`**: every surface feature attached to this solid.
- **`surface_geom`**: the surface's geometry for the requested LoD. A surface modelled only at another LoD is dropped here.
- **`GeometryType = 'MULTIPOLYGON'`**: keeps polygon-based surfaces; script 02 explodes these into individual faces.

---

## Output columns

| Column | Description |
|--------|------------|
| `id` | Auto-generated UUID for this row |
| `lod` | LoD level (2 or 3) |
| `building_feature_id` | Feature id of the Building |
| `owner_feature_id` | Feature id of the solid owner: the Building or one of its BuildingParts |
| `surface_feature_id` | Feature id of the surface |
| `building_object_id` / `surface_object_id` | Stable CityDB object ids of the Building and the surface, carried through all downstream tables |
| `objectclass_id` | Numeric type code (`709` WallSurface, `710` GroundSurface, `712` RoofSurface) |
| `classname` | Human-readable type name |
| `geom` | 3D MULTIPOLYGON geometry of the surface at this LoD |

---

## What comes next

Script 02 takes these MULTIPOLYGON geometries and explodes each one into individual POLYGON faces, because the normal and attribute calculations in script 03 operate on single faces.
