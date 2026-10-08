-- Splits each wall face into geom_party (shared with an attached neighbour) and geom_envelope
-- (exterior), by the overlap test script 03 applies between solids of one building. Runs once over
-- the whole table, since a pair can span batches. Rules: docs/code/sql-pipeline/post-02-party-walls.md.

UPDATE {city2tabula_schema}.{lod_schema}_surface_raw
SET area_party_wall = NULL,
    geom_party = NULL,
    geom_envelope = NULL,
    is_party_wall = FALSE,
    neighbour_object_id = NULL
WHERE area_party_wall IS NOT NULL OR is_party_wall;

WITH pairs AS (
    SELECT b.building_feature_id AS a_id, n.building_feature_id AS b_id, n.object_id AS b_object_id
    FROM {city2tabula_schema}.{lod_schema}_building b
    CROSS JOIN LATERAL unnest(b.attached_neighbour_id) AS nid(object_id)
    JOIN {city2tabula_schema}.{lod_schema}_building n ON n.object_id = nid.object_id
),
walls AS (
    SELECT id, building_feature_id, normal_x AS nx, normal_y AS ny, normal_z AS nz, geom, geom_exposed
    FROM {city2tabula_schema}.{lod_schema}_surface_raw
    WHERE classname = 'WallSurface'
      AND normal_x IS NOT NULL
      AND (geom_exposed IS NULL OR NOT ST_IsEmpty(geom_exposed))
),
shared AS (
    SELECT a.id, p.b_object_id,
           ST_CollectionExtract(ST_Intersection(
               ST_ReducePrecision(ST_MakeValid(ST_Force2D(ra.g)), 0.001),
               ST_ReducePrecision(ST_MakeValid(ST_Force2D(rb.g)), 0.001)
           ), 3) AS geom_2d
    FROM pairs p
    JOIN walls a ON a.building_feature_id = p.a_id
    JOIN walls b ON b.building_feature_id = p.b_id
     AND ABS(a.nx * b.nx + a.ny * b.ny + a.nz * b.nz) >= 0.999
     AND ST_3DDWithin(a.geom, b.geom, 0.1)
    CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(COALESCE(a.geom_exposed, a.geom), a.nx, a.ny, a.nz) AS g) ra
    CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(COALESCE(b.geom_exposed, b.geom), a.nx, a.ny, a.nz) AS g) rb
    WHERE ABS((ST_ZMin(rb.g) + ST_ZMax(rb.g)) - (ST_ZMin(ra.g) + ST_ZMax(ra.g))) / 2 <= 0.1
),
per_face AS (
    SELECT id,
           ST_Union(geom_2d) AS shared_2d,
           (ARRAY_AGG(b_object_id ORDER BY ST_Area(geom_2d) DESC))[1] AS neighbour_object_id
    FROM shared
    WHERE NOT ST_IsEmpty(geom_2d)
    GROUP BY id
),
split AS (
    SELECT w.id, w.nx, w.ny, w.nz, pf.neighbour_object_id,
           (ST_ZMin(r.g) + ST_ZMax(r.g)) / 2 AS z,
           -- Same clean-up as script 03: drop pieces narrower than 0.04 m or under 0.01 m2.
           (SELECT ST_Collect(d.geom)
            FROM ST_Dump(ST_Buffer(ST_Buffer(
                   ST_Intersection(ST_ReducePrecision(ST_MakeValid(ST_Force2D(r.g)), 0.001), pf.shared_2d),
                   -0.02, 'join=mitre'), 0.02, 'join=mitre')) d
            WHERE ST_Area(d.geom) >= 0.01) AS party_2d,
           (SELECT ST_Collect(d.geom)
            FROM ST_Dump(ST_Buffer(ST_Buffer(
                   ST_Difference(ST_ReducePrecision(ST_MakeValid(ST_Force2D(r.g)), 0.001), pf.shared_2d),
                   -0.02, 'join=mitre'), 0.02, 'join=mitre')) d
            WHERE ST_Area(d.geom) >= 0.01) AS envelope_2d
    FROM per_face pf
    JOIN walls w ON w.id = pf.id
    CROSS JOIN LATERAL (SELECT {city2tabula_schema}.face_to_plane(COALESCE(w.geom_exposed, w.geom), w.nx, w.ny, w.nz) AS g) r
)
UPDATE {city2tabula_schema}.{lod_schema}_surface_raw sr
SET area_party_wall = ROUND(ST_Area(s.party_2d)::numeric, 2),
    geom_party = ST_Multi({city2tabula_schema}.face_from_plane(s.party_2d, s.z, s.nx, s.ny, s.nz)),
    geom_envelope = COALESCE(
        ST_Multi({city2tabula_schema}.face_from_plane(s.envelope_2d, s.z, s.nx, s.ny, s.nz)),
        ST_SetSRID('MULTIPOLYGON Z EMPTY'::geometry, {srid})
    ),
    is_party_wall = TRUE,
    neighbour_object_id = s.neighbour_object_id
FROM split s
WHERE sr.id = s.id
  AND s.party_2d IS NOT NULL;
