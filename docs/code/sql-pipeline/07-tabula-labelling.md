---
audience: developer
---

# Post script 02: TABULA Labelling

**File:** `sql/scripts/post/02_label_buildings.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`, `{city2tabula_schema}.tabula_variant`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Assigns each building its best-matching TABULA archetype (variant code) by finding the **nearest neighbour** in a 9-dimensional feature space. It runs once over the whole table after every batch and after [neighbour detection](post-01-neighbour-detection.md), because the normalisation spans all buildings.

TABULA (Typology Approach for Building Stock Energy Assessment) defines a set of reference building archetypes for each country, characterised by attributes like volume, floor area, and storey count. This script maps each extracted building to the archetype it most closely resembles.

---

## Background: nearest-neighbour matching in feature space

Think of each building and each TABULA variant as a point in 9-dimensional space, where each axis represents one building attribute (volume, footprint area, storeys, etc.). The matching question is: *which TABULA variant point is closest to this building point?*

The distance is the root mean square of the per-dimension differences, taken over the dimensions both points know:

```
distance = sqrt( mean over known dimensions of (building.x - variant.x)² )
```

The variant with the smallest distance is the best match. TABULA leaves many variant values empty, for example the volume of every Dutch variant. Averaging over the known dimensions, as in Gower's (1971) similarity coefficient, keeps a variant with fewer known values from winning only because its sum has fewer terms.

---

## Background: why normalise?

The 9 features have very different scales. Volume is measured in cubic metres and might range from hundreds to tens of thousands. Footprint complexity is a 0–2 integer code. If these are used as-is, volume would dominate the distance calculation simply because its numbers are larger: a 1-unit difference in complexity would be invisible next to a 1,000-unit difference in volume.

**Min-max normalisation** rescales every feature to the range [0, 1]:

```
normalised_value = (raw_value - global_min) / (global_max - global_min)
```

After normalisation, a difference of 1.0 in any dimension means spanning the full range of that feature. All dimensions contribute equally to the distance.

---

## CTE walkthrough

### Step 1: `stats`

```sql
WITH stats AS (
  SELECT
    MIN(max_volume) AS min_vol, MAX(max_volume) AS max_vol,
    MIN(footprint_area) AS min_area, MAX(footprint_area) AS max_area,
    ...
  FROM (
    SELECT ... FROM {lod_schema}_building WHERE ...
    UNION ALL
    SELECT ... FROM tabula_variant WHERE ...
  ) all_data
)
```

Computes the global minimum and maximum for each of the 9 features across **both** buildings and TABULA variants combined.

**Why combine them?** If the normalisation range is computed from buildings only, variants may fall outside [0, 1] (if any variant has a larger volume than any extracted building, for example). Using the combined range ensures both sides are scaled to the same axis, making cross-table Euclidean distances meaningful.

The 9 features used are:

| Feature | What it measures |
|---------|----------------|
| `min_volume` | Eave height × footprint, compared with TABULA's conditioned volume `V_C`, which leaves out an unheated attic |
| `footprint_area` | Ground floor area |
| `number_of_storeys` | Storey count |
| `footprint_complexity` | 0–2 shape complexity code |
| `roof_complexity` | 0–2 roof shape code |
| `attached_neighbour_class` | 0 alone, 1 one neighbour, 2 two or more ([neighbour detection](post-01-neighbour-detection.md)) |
| `area_total_roof` | Total roof surface area |
| `area_total_wall` | Total wall surface area |
| `area_total_floor` | Total floor area (all storeys) |

---

### Step 2: `ranked`

```sql
ranked AS (
  SELECT b.building_feature_id,
         v.tabula_variant_code_id,
         v.tabula_variant_code,
         ROW_NUMBER() OVER (PARTITION BY b.building_feature_id
                            ORDER BY d.distance, v.tabula_variant_code_id) AS rnk
  FROM buildings b
  CROSS JOIN tabula_variant v
  CROSS JOIN stats s
  CROSS JOIN LATERAL (
    SELECT sqrt(avg(term)) AS distance
    FROM (VALUES
      (power(minmax_norm(b.max_volume, s.lo_vol, s.hi_vol)  -- b.max_volume is the building's min_volume
           - minmax_norm(v.max_volume, s.lo_vol, s.hi_vol), 2)),
      ... (8 more dimensions)
    ) terms(term)
  ) d
  WHERE d.distance IS NOT NULL
)
```

This CTE compares **every building against every TABULA variant** using a `CROSS JOIN`. For each (building, variant) pair, the distance is computed over the dimensions both sides know.

`ROW_NUMBER()` ranks all variants for each building by distance (closest first). The building is partitioned (`PARTITION BY b.building_feature_id`) so ranks restart at 1 for each building independently.

**Handling NULLs and zero ranges:**

- `minmax_norm(x, lo, hi)` (`sql/functions/03_minmax_norm.sql`) returns NULL when `x` is unknown or the range is empty (all values equal). That dimension's term is NULL, and `avg` skips it.
- The TABULA extraction stores TABULA's missing values (0 or empty) as NULL, so a missing variant value is never compared as zero.
- `minmax_norm` divides in double precision, so the integer codes (storeys, complexity, attached neighbours) normalise to fractions instead of flooring to 0.
- A pair with no known dimension in common has no distance and is not ranked. Ties go to the lower `tabula_variant_code_id`.

---

### Step 3: UPDATE

```sql
UPDATE {lod_schema}_building bf
SET tabula_variant_code_id = ranked.tabula_variant_code_id,
    tabula_variant_code    = ranked.tabula_variant_code
FROM ranked
WHERE bf.building_feature_id = ranked.building_feature_id
  AND ranked.rnk = 1
```

For each building, takes only the rank-1 variant (the closest one) and writes its code back into `_building`.

---

## Output

After this script, each row in `_building` has:

| Column | Description |
|--------|------------|
| `tabula_variant_code_id` | Numeric ID of the matched TABULA variant |
| `tabula_variant_code` | Human-readable TABULA variant code (e.g. `DE.N.SFH.04.Gen`) |

These codes are the primary output of the City2TABULA pipeline and are used downstream for energy demand estimation.

---

## What comes next

This is the last script that writes building attributes. `_building` now holds a fully populated row for every building: geometry-derived attributes, height, area, volume, storey count, shape complexity and a TABULA archetype assignment.

Script 08, which runs per batch before the post scripts, writes the resolved surface table: one row per polygon face, party walls excluded.

---

## Reference

Gower, J. C. (1971). A general coefficient of similarity and some of its properties. *Biometrics* 27(4), 857-871. <https://doi.org/10.2307/2528823>
