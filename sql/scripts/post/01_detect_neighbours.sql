-- Marks the buildings that share a wall with another building. Runs once over the
-- whole _building table after every batch has finished: neighbours cross batch
-- boundaries, so a per-batch step would miss pairs.
--
-- Two buildings are attached when their footprints lie within 0.1 m of each other
-- and share at least 1 m of boundary, which leaves out corner-only contacts.
-- 0.1 m separates touching from detached footprints in the NL, DE, AT and CZ
-- datasets alike; wider gaps are mostly detached houses and sheds.
--
-- attached_neighbour_id lists the neighbours' object_ids, which stay stable across
-- re-imports, unlike building_feature_id.
-- attached_neighbour_class follows TABULA's Code_AttachedNeighbours: 0 alone,
-- 1 one neighbour, 2 two or more. A building without a footprint stays NULL.
-- Only changed rows are written, so the correction triggers fire only for them.

-- pairs joins the table, not a CTE: a CTE referenced twice is materialised and the
-- join then cannot use the footprint GIST index.
WITH pairs AS (
    SELECT a.building_feature_id AS a_id, c.building_feature_id AS c_id,
           a.object_id AS a_obj, c.object_id AS c_obj
    FROM {city2tabula_schema}.{lod_schema}_building a
    JOIN {city2tabula_schema}.{lod_schema}_building c
      ON a.building_feature_id < c.building_feature_id
     AND ST_DWithin(a.building_footprint_geom, c.building_footprint_geom, 0.1)
    WHERE ST_Length(ST_Intersection(ST_Boundary(ST_Force2D(a.building_footprint_geom)),
                                    ST_Buffer(ST_Force2D(c.building_footprint_geom), 0.1))) >= 1
),
neighbours AS (
    SELECT id, ARRAY_AGG(other ORDER BY other)::TEXT[] AS ids
    FROM (
        SELECT a_id AS id, c_obj AS other FROM pairs
        UNION ALL
        SELECT c_id, a_obj FROM pairs
    ) p
    GROUP BY id
),
detected AS (
    SELECT b.building_feature_id AS id, COALESCE(n.ids, ARRAY[]::TEXT[]) AS ids
    FROM {city2tabula_schema}.{lod_schema}_building b
    LEFT JOIN neighbours n ON n.id = b.building_feature_id
    WHERE b.building_footprint_geom IS NOT NULL
)
UPDATE {city2tabula_schema}.{lod_schema}_building t
SET has_attached_neighbour   = cardinality(d.ids) > 0,
    attached_neighbour_id    = d.ids,
    total_attached_neighbour = cardinality(d.ids),
    attached_neighbour_class = LEAST(cardinality(d.ids), 2)
FROM detected d
WHERE t.building_feature_id = d.id
  AND (t.attached_neighbour_id, t.attached_neighbour_class)
      IS DISTINCT FROM (d.ids, LEAST(cardinality(d.ids), 2));
