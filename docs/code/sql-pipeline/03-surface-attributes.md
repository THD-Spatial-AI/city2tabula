# Script 03: Surface Attributes

**File:** `sql/scripts/main/03_calc_child_feat_attr.sql`

**Reads from:** `{city2tabula_schema}.{lod_schema}_child_feature_geom_dump`

**Writes to:** `{city2tabula_schema}.{lod_schema}_child_feature_surface`

---

## Purpose

For each individual polygon face (from script 02), this script computes four physical attributes:

- **Tilt**: how steeply the surface is inclined (0° = flat horizontal roof, 90° = vertical wall).
- **Azimuth**: the compass direction the surface faces (0°/360° = North, 90° = East, 180° = South, 270° = West).
- **Surface area**: the true 3D face area in square metres.
- **Height span**: the vertical range of the face (Z max − Z min) in metres.

All four attributes are derived from the **surface normal**: a vector that points perpendicularly outward from the face. The bulk of this script's CTEs are dedicated to computing that normal correctly.

---

## Background: what is a surface normal?

Imagine a flat wall. If you placed a stick perpendicular to the wall and pointing away from the building, that stick is the surface normal. It encodes two things:

- Its **vertical component** (how much it points up or down) determines tilt.
- Its **horizontal direction** (which compass bearing it points toward) determines azimuth.

For the normal to be useful, it must:

1. Be **correctly computed** from the polygon's vertices.
2. Point **outward** (away from the building), not inward.

Both of these requirements require care when working with real-world CityGML data, which is why script 03 has eight CTEs.

---

## Background: Newell's method

### Why not a simple cross product to compute the normal vector?

The simplest way to compute a surface normal is to take three polygon vertices and compute their cross product. For a flat (planar) polygon this works, but CityGML surfaces are sometimes slightly non-planar due to data quality issues (vertices don't lie perfectly on a plane). Picking an arbitrary triplet on a warped surface gives an inconsistent result.

### Newell's method

Newell's method solves this by accumulating a contribution from every consecutive edge pair around the polygon ring:

```
n_x = SUM over all edges: (y_i − y_{i+1}) × (z_i + z_{i+1})
n_y = SUM over all edges: (z_i − z_{i+1}) × (x_i + x_{i+1})
n_z = SUM over all edges: (x_i − x_{i+1}) × (y_i + y_{i+1})
```

For a perfectly flat polygon this gives the same answer as a simple cross product. For a warped polygon it gives the best-fit normal that minimises the error across all vertices: equivalent to fitting a plane by least squares. It runs in O(n) time (one pass over the edges) and handles both cases uniformly.

---

## Background: vertex winding order

The sign of a cross-product normal depends on the order in which vertices are listed. If you trace the polygon boundary clockwise, the normal points one way; counter-clockwise, the opposite way. CityGML datasets from different providers may use either convention, and sometimes even mix them. Without correction, a north-facing wall might be computed as south-facing (180° error) purely because its vertices happen to be listed in a different order.

This script corrects for this by comparing the computed normal against a known reference point inside the building (the **interior point**, derived from the GroundSurface). If the normal points toward the interior rather than away from it, it is flipped.

---

## CTE walkthrough

### Step 1: `new_buildings`

```sql
SELECT DISTINCT building_feature_id
FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump
WHERE building_feature_id IN {building_ids}
  AND building_feature_id NOT IN (
    SELECT DISTINCT building_feature_id
    FROM {city2tabula_schema}.{lod_schema}_child_feature_surface
  )
```

Idempotency guard: selects only buildings not yet present in the output table.

---

### Step 2: `building_interior_pts`

```sql
SELECT
  building_feature_id,
  ST_PointOnSurface(ST_Collect(ST_Force2D(geom))) AS interior_pt
FROM ...
WHERE classname = 'GroundSurface'
GROUP BY building_feature_id
```

For each building, computes a single 2D reference point guaranteed to lie **inside** the ground footprint.

- `ST_Force2D` strips Z coordinates: we only need the horizontal position.
- `ST_Collect` merges all GroundSurface polygons into one geometry without dissolving them.
- `ST_PointOnSurface` returns a point on the surface that is guaranteed to be inside, even for non-convex shapes like L-shaped footprints (where the centroid can fall outside).

This point is used in `oriented_normals` to check whether a wall's computed normal points outward or inward.

---

### Step 3: `raw_surfaces`

```sql
SELECT
  ...,
  gd.geom AS valid_geom,
  ST_IsPlanar(gd.geom) AS is_planar
FROM {city2tabula_schema}.{lod_schema}_child_feature_geom_dump gd
INNER JOIN new_buildings nb ON gd.building_feature_id = nb.building_feature_id
```

One row per polygon face from the geometry dump. `ST_IsPlanar` checks whether all vertices lie on a single plane. This flag is stored in the output for diagnostic purposes but does not change how the normal is computed: Newell's method handles both planar and non-planar polygons identically.

---

### Step 4: `surface_points`

```sql
SELECT
  *,
  (ST_DumpPoints(valid_geom)).geom AS point_geom,
  (ST_DumpPoints(valid_geom)).path[2] AS pt_idx
FROM raw_surfaces
```

`ST_DumpPoints` extracts every individual vertex from the polygon ring as a separate row. `path[2]` gives the sequential index of each vertex within the ring, so they can be walked in order.

**Example:** a quadrilateral face with vertices v1, v2, v3, v4 (plus v1 again to close the ring) produces five rows with `pt_idx` values 1, 2, 3, 4, 5.

---

### Step 5: `surface_edges`

```sql
SELECT
  ...,
  point_geom,
  LEAD(point_geom) OVER (PARTITION BY id ORDER BY pt_idx) AS next_pt
FROM surface_points
```

`LEAD` is a window function that pairs each vertex row with the *next* vertex in the sequence. This turns the list of vertices into a list of directed edges (current_vertex → next_vertex). The last vertex in the ring is paired with `NULL` (there is no next vertex), which is filtered out in the next step.

**Result:** each row now represents one edge, described by its start and end point, ready for Newell's formula.

---

### Step 6: `surface_normals`

```sql
SELECT
  ...,
  SUM((ST_Y(point_geom) - ST_Y(next_pt)) * (ST_Z(point_geom) + ST_Z(next_pt))) AS n_x,
  SUM((ST_Z(point_geom) - ST_Z(next_pt)) * (ST_X(point_geom) + ST_X(next_pt))) AS n_y,
  SUM((ST_X(point_geom) - ST_X(next_pt)) * (ST_Y(point_geom) + ST_Y(next_pt))) AS n_z
FROM surface_edges
WHERE next_pt IS NOT NULL
GROUP BY id, ...
HAVING COUNT(*) >= 2
```

Applies Newell's formula across all edges of each polygon face. The three sums give the raw (un-normalised) normal vector (n_x, n_y, n_z).

The outer query then computes the vector's **magnitude** (`sqrt(n_x² + n_y² + n_z²)`) and discards surfaces where the magnitude is near zero, which means all vertices are collinear (i.e. the polygon has no area and no well-defined normal).

---

### Step 7: `oriented_normals`

```sql
COALESCE(
  CASE
    WHEN (centroid_x - interior_x) * (n_x / magnitude)
       + (centroid_y - interior_y) * (n_y / magnitude)
    >= 0 THEN 1.0
    ELSE -1.0
  END,
  1.0
) AS surface_flip
```

For each `WallSurface` and `RoofSurface`, computes a **dot product** between:
- The horizontal component of the surface normal (nx, ny), and
- The vector from the building's interior point to the surface centroid.

If the dot product is ≥ 0, the normal already points away from the interior (outward): `surface_flip = +1`, keep as-is. If the dot product is negative, the normal points inward: `surface_flip = −1`, flip it.

If no interior point exists (the building has no GroundSurface), `COALESCE` falls back to `+1` (no flip).

---

### Step 8: `normalized_normals`

```sql
CASE
  WHEN classname = 'RoofSurface' AND (n_z / magnitude) < 0
    THEN -(n_x / magnitude)
  WHEN classname = 'WallSurface'
    THEN surface_flip * (n_x / magnitude)
  ELSE n_x / magnitude
END AS nx
```

Applies per-class rules to produce the final unit normal (nx, ny, nz):

- **RoofSurface:** if the Z component is negative (normal points downward), flip all three components. A roof normal must always point upward. The Z sign check is used here rather than the dot-product flip because, for near-flat roofs, the horizontal components (nx, ny) are tiny and sign-unstable.
- **WallSurface:** multiply all three components by `surface_flip` from the previous step. This corrects the CW/CCW winding issue.
- **Other types:** divide by magnitude to get a unit vector, no flip.

---

## Final SELECT: computing the attributes

With the unit normal `(nx, ny, nz)` available, the attributes are:

```sql
-- Tilt: angle of the normal above the horizontal plane
DEGREES(ASIN(ABS(nz))) AS tilt
```

`nz` is the vertical component of the unit normal. For a vertical wall, `nz = 0`, giving `ASIN(0) = 0°`. For a flat horizontal surface, `|nz| = 1`, giving `ASIN(1) = 90°`.

```sql
-- Azimuth: compass bearing of the horizontal normal component
CASE
  WHEN ABS(nz) > 0.985 THEN -1  -- surface is near-horizontal; azimuth is undefined
  ELSE MOD((450.0 - degrees(atan2(ny, nx))) + 360.0, 360.0)
END AS azimuth
```

`atan2(ny, nx)` computes the mathematical angle (counter-clockwise from East). The formula `(450 − angle) mod 360` converts this to a compass bearing (clockwise from North). For nearly-flat surfaces (tilt > ~80°), the horizontal components are near zero and `atan2` becomes numerically unreliable, so azimuth is set to `-1` (undefined).

```sql
-- Surface area: projected onto the surface plane before measurement
{city2tabula_schema}.surface_area_corrected_geom(valid_geom, nx, ny, nz)
```

`ST_Area(geom)` measures area on the XY plane, which underestimates a tilted surface (the same way a tilted roof looks smaller from directly above than its true size). The `surface_area_corrected_geom` function projects the polygon onto its own surface plane before measuring, giving the true 3D area.

```sql
-- Height: vertical span of the face
(ST_ZMax(valid_geom) - ST_ZMin(valid_geom)) AS height
```

The simplest attribute: the difference between the highest and lowest Z coordinate of the polygon's vertices.

---

## Output columns

| Column | Description |
|--------|------------|
| `surface_area` | True 3D area of the face (sqm) |
| `tilt` | Inclination from horizontal (degrees; 0° = wall, 90° = flat roof) |
| `azimuth` | Compass bearing of outward-facing normal (degrees; −1 = undefined) |
| `is_valid` | `ST_IsValid` result for the polygon |
| `is_planar` | Whether all vertices lie on a single plane |
| `height` | Vertical span: ZMax − ZMin (m) |
| `geom` | Original polygon geometry (carried through) |

---

## What comes next

Script 04 reads `_child_feature_surface` and aggregates the per-face attributes into a single summary row per building.
