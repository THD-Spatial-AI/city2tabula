---
audience: developer
---

# Post scripts 02 to 04: Party Walls

**Files:** `sql/scripts/post/02_detect_party_walls.sql`, `03_build_surface.sql`, `04_wall_totals.sql`  
**Reads from:** `{city2tabula_schema}.{lod_schema}_building`, `{city2tabula_schema}.{lod_schema}_surface_raw`  
**Writes to:** `_surface_raw` (UPDATE), `_surface` (rebuilt), `_building` (UPDATE)

---

## Purpose

A party wall is the part of a wall that lies against a wall of an attached neighbour. With a heated neighbour on the other side there is almost no temperature difference across it, so it hardly transmits heat. TABULA counts only the heat-transmitting envelope in its wall areas, and BuEM models such a wall with `b_transmission = 0`. These scripts find the shared part of every wall, serve it separately, and leave it out of the building's wall area.

They run once over the whole table after [neighbour detection](post-01-neighbour-detection.md), because the two buildings of a pair can sit in different batches.

---

## Detection (post script 02)

For each building and each of its attached neighbours (`attached_neighbour_id`), a wall face of the building is shared where a wall face of the neighbour lies on it. The test is the one script 03 applies to faces between solids of one building:

| Condition | Value |
|-----------|-------|
| Normals parallel | `|cos| ≥ 0.999`, about 2.6° |
| Distance in 3D | at most 0.1 m, the footprint tolerance of neighbour detection |
| Plane offset | at most 0.1 m, read in the face's own plane (`face_to_plane`) |
| Overlap | 2D intersection in that plane; pieces narrower than 0.04 m or under 0.01 m² are dropped |

The test starts from what script 03 left exterior, so a face already internal to its own building is not counted again. Each face that shares any area gets:

| Column | Meaning |
|--------|---------|
| `is_party_wall` | `TRUE` |
| `area_party_wall` | Shared area (m²) |
| `geom_party` | Shared pieces, in 3D |
| `geom_envelope` | Exterior pieces, in 3D; empty when the whole face is shared |
| `neighbour_object_id` | The neighbour sharing the most area |

Every run resets these columns and recomputes them, so a wall becomes exterior again when its neighbour is removed.

---

## Served surfaces (post script 03)

`_surface` is rebuilt from `_surface_raw` after every extraction. Each face is served as pieces:

- **Exterior pieces** (`is_party_wall = FALSE`): `geom_envelope` when the face has a party wall, otherwise `geom_exposed` when part of it is internal (script 03), otherwise the face itself.
- **Party-wall pieces** (`is_party_wall = TRUE`): `geom_party`, with `neighbour_object_id`.

Area, height, length and width are measured on each piece; tilt and azimuth are those of the face. A wall that is only partly shared, for example a taller building's wall above its neighbour's roof, is served as one exterior and one party-wall piece.

Each side of a shared wall has its own polygon, so the two buildings' party-wall pieces lie on top of each other. A viewer highlights the selected building's pieces only.

---

## Wall totals (post script 04)

| Column | Value |
|--------|-------|
| `area_total_wall` | Sum of the exterior wall pieces |
| `area_party_wall` | Sum of the party-wall pieces |
| `surface_count_wall` | Number of exterior wall pieces |

TABULA matching ([post script 05](07-tabula-labelling.md)) runs after this step, so it compares envelope walls with TABULA's envelope walls.

!!! warning "Heated neighbours assumed"
    Leaving a party wall out of the envelope is right when the neighbour is heated. A wall shared with an unheated building, such as a garage or shed, still loses heat, at a reduced rate. City2TABULA does not judge whether a neighbour is heated: `area_party_wall` and the party-wall pieces with `neighbour_object_id` let a consumer add such a wall back with its own reduction factor.
