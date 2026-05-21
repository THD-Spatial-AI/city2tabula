# Script 05 — Volume

**File:** `sql/scripts/main/05_calc_volume.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building_feature`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building_feature` (UPDATE)

---

## Purpose

Estimates building volume using a simple bounding-box approximation: **height × footprint area**. Two volume estimates are computed — one using the eave height and one using the ridge height — providing a lower and upper bound on the true volume.

This is a deliberate simplification. Computing the exact volume of a 3D building solid from CityGML would require expensive geometric operations. For the purposes of TABULA archetype matching (script 07), a height × footprint approximation is sufficiently discriminating and much faster to compute.

---

## What it does

```sql
UPDATE {city2tabula_schema}.{lod_schema}_building_feature AS bf
SET
    min_volume = bf.min_height * bf.footprint_area,
    max_volume = bf.max_height * bf.footprint_area,
    min_volume_unit = 'cbm',
    max_volume_unit = 'cbm'
WHERE bf.building_feature_id IN {building_ids}
```

This is a straightforward UPDATE. No CTEs are needed because both operands (`min_height`, `max_height`, `footprint_area`) are already columns in the same row from script 04.

The `CASE` expressions guard against NULL inputs: if either operand is NULL (e.g. a building with no wall surfaces), the column is left unchanged rather than overwritten with NULL.

---

## Volume semantics

| Column | Formula | Interpretation |
|--------|---------|----------------|
| `min_volume` | `min_height × footprint_area` | Eave-height box — excludes the roof volume. Conservative lower bound. |
| `max_volume` | `max_height × footprint_area` | Ridge-height box — includes the full roof height as if it were a box. Liberal upper bound. |

The true building volume lies somewhere between these two values. For a building with a flat roof, `min_volume` and `max_volume` are equal.

---

## What comes next

Script 06 refines the storey count and overwrites the total floor area estimate.
