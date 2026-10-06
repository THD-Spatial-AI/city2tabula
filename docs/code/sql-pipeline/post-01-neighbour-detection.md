---
audience: developer
---

# Post script 01: Neighbour Detection

**File:** `sql/scripts/post/01_detect_neighbours.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`  
**Writes to:** `{city2tabula_schema}.{lod_schema}_building` (UPDATE)

---

## Purpose

Marks the buildings that share a wall with another building, and counts those neighbours. The count is one of the dimensions [TABULA labelling](07-tabula-labelling.md) matches on, because TABULA types distinguish detached, end-terrace and mid-terrace houses.

It runs once over the whole table after every batch has finished, not per batch. Neighbours cross batch boundaries, so a batch on its own does not see all of them.

---

## Rule

Two buildings of the same LoD are attached when both of these hold:

| Condition | Value | Effect |
|-----------|-------|--------|
| Footprint gap | at most 0.1 m | Footprints that touch, allowing for small digitising gaps |
| Shared boundary length | at least 1 m | Excludes buildings that meet only at a corner |

The footprint is `building_footprint_geom` from [script 04](04-building-features.md), compared in 2D. A building without a footprint keeps NULL in every column below.

!!! info "Choice of 0.1 m"
    In the Dutch, German, Austrian and Czech test datasets almost every pair within 0.1 m shares at least 3 m of boundary. Pairs between 0.1 m and 1 m apart are few, and most share little boundary: detached houses and outbuildings with a narrow gap.

---

## Output

| Column | Description |
|--------|------------|
| `has_attached_neighbour` | `TRUE` when the building has at least one attached neighbour |
| `attached_neighbour_id` | `object_id` of each attached neighbour, ascending |
| `total_attached_neighbour` | Number of attached neighbours |
| `attached_neighbour_class` | TABULA `Code_AttachedNeighbours`: 0 alone, 1 one neighbour, 2 two or more |

`GET /api/v1/buildings` serves `has_attached_neighbour`, `attached_neighbour_class` and `attached_neighbour_id`; pass the ids to `GET /api/v1/geometry` to draw the neighbours.

Only rows whose values change are written. A footprint corrected by hand does not re-run this script; run `-extract-features` again after importing new buildings.
