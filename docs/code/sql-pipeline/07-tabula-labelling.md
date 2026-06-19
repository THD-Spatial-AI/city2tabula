# Script 07 — TABULA Labelling

**File:** `sql/scripts/main/07_label_buildings.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`, `{city2tabula_schema}.tabula_variant`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Assigns each building its best-matching TABULA archetype (variant code) by finding the **nearest neighbour** in an 8-dimensional feature space. This is the final step of the extraction pipeline.

TABULA (Typology Approach for Building Stock Energy Assessment) defines a set of reference building archetypes for each country, characterised by attributes like volume, floor area, and storey count. This script maps each extracted building to the archetype it most closely resembles.

---

## Background: nearest-neighbour matching in feature space

Think of each building and each TABULA variant as a point in 8-dimensional space, where each axis represents one building attribute (volume, footprint area, storeys, etc.). The matching question is: *which TABULA variant point is closest to this building point?*

The distance between two points is the standard Euclidean formula, extended to 8 dimensions:

```
distance = sqrt(
  (building.volume - variant.volume)² +
  (building.area   - variant.area  )² +
  ... (5 more dimensions)
)
```

The variant with the smallest distance is the best match.

---

## Background: why normalise?

The 8 features have very different scales. Volume is measured in cubic metres and might range from hundreds to tens of thousands. Footprint complexity is a 0–2 integer code. If these are used as-is, volume would dominate the distance calculation simply because its numbers are larger — a 1-unit difference in complexity would be invisible compared to a 1,000-unit difference in volume.

**Min-max normalisation** rescales every feature to the range [0, 1]:

```
normalised_value = (raw_value - global_min) / (global_max - global_min)
```

After normalisation, a difference of 1.0 in any dimension means spanning the full range of that feature. All dimensions contribute equally to the distance.

---

## CTE walkthrough

### Step 1 — `stats`

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

Computes the global minimum and maximum for each of the 8 features across **both** buildings and TABULA variants combined.

**Why combine them?** If the normalisation range is computed from buildings only, variants may fall outside [0, 1] (if any variant has a larger volume than any extracted building, for example). Using the combined range ensures both sides are scaled to the same axis, making cross-table Euclidean distances meaningful.

The 8 features used are:

| Feature | What it measures |
|---------|----------------|
| `max_volume` | Upper-bound volume (ridge height × footprint) |
| `footprint_area` | Ground floor area |
| `number_of_storeys` | Storey count |
| `footprint_complexity` | 0–2 shape complexity code |
| `roof_complexity` | 0–2 roof shape code |
| `area_total_roof` | Total roof surface area |
| `area_total_wall` | Total wall surface area |
| `area_total_floor` | Total floor area (all storeys) |

---

### Step 2 — `ranked`

```sql
ranked AS (
  SELECT b.building_feature_id,
         v.tabula_variant_code_id,
         v.tabula_variant_code,
         ROW_NUMBER() OVER (
           PARTITION BY b.building_feature_id
           ORDER BY sqrt(
             power(normalised_building_volume - normalised_variant_volume, 2) +
             power(normalised_building_area   - normalised_variant_area,   2) +
             ... (6 more terms)
           ) ASC
         ) AS rnk
  FROM {lod_schema}_building b
  CROSS JOIN tabula_variant v
  CROSS JOIN stats s
  WHERE ...
)
```

This CTE compares **every building against every TABULA variant** using a `CROSS JOIN`. For each (building, variant) pair, the normalised Euclidean distance is computed across all 8 dimensions.

`ROW_NUMBER()` ranks all variants for each building by distance (closest first). The building is partitioned (`PARTITION BY b.building_feature_id`) so ranks restart at 1 for each building independently.

**Handling NULLs and zero ranges:**

- `COALESCE(..., 0)` — if a building or variant has a NULL value for a feature (e.g. missing roof data), that dimension is treated as sitting at the normalised minimum (0). This keeps the distance computation valid without discarding rows.
- `NULLIF(range, 0)` — if the global max equals the global min for a feature (all values are identical, so range = 0), division would produce an error. `NULLIF` converts 0 to NULL, making the division produce NULL, which `COALESCE` then converts to 0. The practical effect: a feature with zero discriminating power contributes nothing to the distance.

---

### Step 3 — UPDATE

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

## Pipeline complete

This is the last of the seven extraction scripts. At this point, `_building` contains a fully populated row for every building in the batch: geometry-derived attributes, height, area, volume, storey count, shape complexity, and a TABULA archetype assignment.
