---
audience: developer
---

# Script 05: Volume

**File:** `sql/scripts/main/05_calc_volume.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Estimates building volume using a simple bounding-box approximation: **height × footprint area**. Two estimates are computed, one from the eave height and one from the ridge height, giving a lower and an upper bound on the true volume.

This is a deliberate simplification. Computing the exact volume of a 3D building solid from CityGML would require expensive geometric operations. For the purposes of TABULA archetype matching (script 07), a height × footprint approximation is sufficiently discriminating and much faster to compute.

---

## What it does

```sql
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET
    min_volume = CASE
        WHEN bf.min_height IS NOT NULL AND bf.footprint_area IS NOT NULL
        THEN ROUND((bf.min_height * bf.footprint_area)::numeric, 2)
        ELSE bf.min_volume
    END,
    -- max_volume and the two unit columns follow the same shape
    ...
WHERE bf.building_feature_id IN {building_ids}
```

No CTEs are needed: both operands (`min_height`, `max_height`, `footprint_area`) are already columns in the same row, written by script 04.

Each assignment is wrapped in a `CASE` that guards against NULL inputs. When either operand is NULL, for example a building with no wall surfaces, the column keeps its previous value instead of being overwritten with NULL.

---

## Volume semantics

| Column | Formula | Interpretation |
|--------|---------|----------------|
| `min_volume` | `min_height × footprint_area` | Eave-height box, excluding the roof volume. Conservative lower bound. |
| `max_volume` | `max_height × footprint_area` | Ridge-height box, counting the full roof height as if it were a box. Liberal upper bound. |

The true building volume lies somewhere between these two values. For a building with a flat roof, `min_volume` and `max_volume` are equal.

---

## What comes next

Script 06 refines the storey count and overwrites the total floor area estimate.
