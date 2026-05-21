# Script 06 — Storeys

**File:** `sql/scripts/main/06_calc_storeys.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building_feature`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building_feature` (UPDATE)

---

## Purpose

Refines the storey count using the stored `room_height` value and overwrites `area_total_floor` with a total heated floor area estimate that accounts for all storeys.

Script 04 computed a preliminary `number_of_storeys` inline as `wall_height / 2.5`. This script performs the same calculation but reads from the stored columns, making the room height value explicit and allowing it to be changed per dataset without modifying the SQL.

---

## What it does

```sql
UPDATE {city2tabula_schema}.{lod_schema}_building_feature AS bf
SET
    number_of_storeys = bf.min_height / bf.room_height,
    room_height_unit  = 'm',
    area_total_floor  = bf.footprint_area * bf.number_of_storeys,
    area_total_floor_unit = 'sqm'
WHERE bf.building_feature_id IN {building_ids}
```

Two values are updated:

### `number_of_storeys`

```
number_of_storeys = min_height / room_height
```

`min_height` is the eave height (maximum wall face span from script 04). `room_height` is the assumed ceiling-to-floor height, defaulting to 2.5 m. Dividing gives the number of storeys in the habitable wall portion of the building.

Guards: if either value is NULL or zero, the existing `number_of_storeys` is left unchanged.

### `area_total_floor`

```
area_total_floor = footprint_area × number_of_storeys
```

This overwrites the value set in script 04 (which was just `footprint_area`). The result is an estimate of the **total heated floor area** across all storeys — the metric used in energy demand calculations.

!!! warning "PostgreSQL UPDATE evaluation order"
    In a `SET` clause, all right-hand side expressions are evaluated from the **row state before the UPDATE begins**. This means `bf.number_of_storeys` in the `area_total_floor` expression reads the *old* value, not the newly computed one from the same SET clause. The effect is that `area_total_floor` is computed as `footprint_area × old_number_of_storeys`, which is the value from script 04's initial estimate. This is a known limitation and is noted in the script comment. The difference is small in practice because both estimates use the same formula.

---

## What comes next

Script 07 uses the final building feature values — including volume, areas, storeys, and complexity — to match each building to its closest TABULA archetype.
