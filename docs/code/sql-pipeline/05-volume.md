---
audience: developer
---

# Script 05: Volume

**File:** `sql/scripts/main/05_calc_volume.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building_part`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Estimates building volume using a simple bounding-box approximation: **height × footprint area** of each solid, summed over the building's solids. Two estimates are computed, one from the eave height and one from the ridge height, giving a lower and an upper bound on the true volume.

This is a deliberate simplification. Computing the exact volume of a 3D building solid from CityGML would require expensive geometric operations. For the purposes of TABULA archetype matching (script 07), a height × footprint approximation is sufficiently discriminating and much faster to compute.

---

## What it does

```sql
WITH part_volumes AS (
    SELECT building_feature_id,
           SUM(min_height * footprint_area) AS min_volume,
           SUM(max_height * footprint_area) AS max_volume
    FROM {city2tabula_schema}.{lod_schema}_building_part
    WHERE building_feature_id IN {building_ids}
    GROUP BY building_feature_id
)
UPDATE {city2tabula_schema}.{lod_schema}_building AS bf
SET min_volume = CASE WHEN pv.min_volume IS NOT NULL THEN ROUND(pv.min_volume::numeric, 2) ELSE bf.min_volume END,
    -- max_volume and the two unit columns follow the same shape
    ...
FROM part_volumes pv
WHERE bf.building_feature_id = pv.building_feature_id
```

Each assignment is wrapped in a `CASE` that guards against NULL inputs. When the sum is NULL, for example a building with no wall surfaces, the column keeps its previous value instead of being overwritten with NULL.

A building of one solid gets exactly `min_height × footprint_area`. For a building of several solids, the sum keeps a tall part's height off the footprint of a low part beside it; `min_height` on the building row is the tallest solid's (script 04), so `min_height × footprint_area` would overstate the volume.

---

## Volume semantics

| Column | Formula | Interpretation |
|--------|---------|----------------|
| `min_volume` | Σ `min_height × footprint_area` over solids | Eave-height boxes, excluding the roof volume. Conservative lower bound. |
| `max_volume` | Σ `max_height × footprint_area` over solids | Ridge-height boxes, counting the full roof height as if it were a box. Liberal upper bound. |

The true building volume lies somewhere between these two values. For a building with flat roofs, `min_volume` and `max_volume` are equal.

---

## What comes next

Script 06 refines the storey count and overwrites the total floor area estimate.
