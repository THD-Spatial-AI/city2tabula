---
audience: developer
---

# Script 06: Storeys

**File:** `sql/scripts/main/06_calc_storeys.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`, `{city2tabula_schema}.{lod_schema}_building_part`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Refines the storey count using the stored `room_height` value and overwrites `area_total_floor` with a total heated floor area estimate that accounts for all storeys of every solid of the building.

Script 04 computed a preliminary `number_of_storeys` inline as the tallest solid's eave height / 2.5. This script performs the same calculation but reads from the stored columns, making the room height value explicit and allowing it to be changed per dataset without modifying the SQL.

---

## What it does

Two values are updated.

### `number_of_storeys`

```
number_of_storeys = min_height / room_height
```

`min_height` is the tallest solid's eave height (script 04). `room_height` is the assumed ceiling-to-floor height, defaulting to 2.5 m. The column is an integer, so the quotient is rounded.

Guards: if either value is NULL or zero, the existing `number_of_storeys` is left unchanged.

### `area_total_floor`

```
area_total_floor = Σ over solids of footprint_area × (min_height / room_height)
```

Each solid in `_building_part` contributes its own footprint times its own storey count (rounded to an integer, 1 when the eave height or room height is 0 or missing). This overwrites the value set in script 04 (the GroundSurface sum). The result is an estimate of the **total heated floor area**, the metric used in energy demand calculations.

A building of one solid gets exactly `footprint_area × number_of_storeys`. For a building of several solids, `number_of_storeys` is the tallest solid's, so multiplying it by the whole footprint would overstate the floor area.

---

## What comes next

Script 07 uses the final building feature values (volume, areas, storeys and complexity) to match each building to its closest TABULA archetype.
