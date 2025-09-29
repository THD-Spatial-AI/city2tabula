-- Combined neighbour detection and classification update
WITH potential_neighbours AS (
    -- Step 1: Find potential neighbours based on centroid proximity
    SELECT
        bf1.building_feature_id AS building_id,
        bf2.building_feature_id AS neighbour_id
    FROM
        {city2tabula_schema}.{lod_schema}_building_feature AS bf1
    JOIN
        {city2tabula_schema}.{lod_schema}_building_feature AS bf2
    ON
        ST_DWithin(bf1.building_centroid_geom, bf2.building_centroid_geom, 50) -- 50 meters search radius
        AND bf1.building_feature_id != bf2.building_feature_id -- Exclude self
),
related_floors AS (
    -- Step 2: Check GroundSurface relationships for potential neighbours
    SELECT
        pn.building_id,
        pn.neighbour_id
    FROM
        potential_neighbours AS pn
    JOIN
        {city2tabula_schema}.{lod_schema}_child_feature_surface AS fs1
    ON
        pn.building_id = fs1.building_feature_id
    JOIN
        {city2tabula_schema}.{lod_schema}_child_feature_surface AS fs2
    ON
        pn.neighbour_id = fs2.building_feature_id
    WHERE
        fs1.classname = 'GroundSurface' -- Only consider GroundSurface
        AND fs2.classname = 'GroundSurface'
        AND ST_Relate(fs1.geom, fs2.geom, 'F***1****') -- Only consider side-touching geometries
),
aggregated_neighbours AS (
    -- Step 3: Aggregate neighbours and counts for each building
    SELECT
        rf.building_id,
        ARRAY_AGG(DISTINCT rf.neighbour_id) AS attached_neighbour_ids, -- Ensure unique neighbour IDs
        COUNT(DISTINCT rf.neighbour_id) AS total_attached_neighbours -- Count unique neighbour IDs
    FROM
        related_floors AS rf
    GROUP BY
        rf.building_id
)
-- Step 4: Single update for ALL neighbour-related fields
UPDATE {city2tabula_schema}.{lod_schema}_building_feature AS bf
SET
    has_attached_neighbour = (an.total_attached_neighbours > 0),
    attached_neighbour_id = an.attached_neighbour_ids,
    total_attached_neighbour = an.total_attached_neighbours,
    attached_neighbour_class = CASE
        WHEN an.total_attached_neighbours > 1 THEN 2 -- B_N2
        WHEN an.total_attached_neighbours = 1 THEN 1 -- B_N1
        ELSE 0 -- B_Alone (this handles NULL case from LEFT JOIN)
    END
FROM
    aggregated_neighbours AS an
WHERE
    bf.building_feature_id = an.building_id;

-- Update buildings that have no neighbours (not in the CTE result)
UPDATE {city2tabula_schema}.{lod_schema}_building_feature AS bf
SET
    has_attached_neighbour = FALSE,
    attached_neighbour_id = NULL,
    total_attached_neighbour = 0,
    attached_neighbour_class = 0 -- B_Alone
WHERE
    bf.building_feature_id NOT IN (
        SELECT building_id FROM (
            WITH potential_neighbours AS (
                SELECT
                    bf1.building_feature_id AS building_id,
                    bf2.building_feature_id AS neighbour_id
                FROM
                    {city2tabula_schema}.{lod_schema}_building_feature AS bf1
                JOIN
                    {city2tabula_schema}.{lod_schema}_building_feature AS bf2
                ON
                    ST_DWithin(bf1.building_centroid_geom, bf2.building_centroid_geom, 50)
                    AND bf1.building_feature_id != bf2.building_feature_id
            ),
            related_floors AS (
                SELECT
                    pn.building_id,
                    pn.neighbour_id
                FROM
                    potential_neighbours AS pn
                JOIN
                    {city2tabula_schema}.{lod_schema}_child_feature_surface AS fs1
                ON
                    pn.building_id = fs1.building_feature_id
                JOIN
                    {city2tabula_schema}.{lod_schema}_child_feature_surface AS fs2
                ON
                    pn.neighbour_id = fs2.building_feature_id
                WHERE
                    fs1.classname = 'GroundSurface'
                    AND fs2.classname = 'GroundSurface'
                    AND ST_Relate(fs1.geom, fs2.geom, 'F***1****')
            ),
            aggregated_neighbours AS (
                SELECT
                    rf.building_id,
                    COUNT(DISTINCT rf.neighbour_id) AS total_attached_neighbours
                FROM
                    related_floors AS rf
                GROUP BY
                    rf.building_id
            )
            SELECT building_id FROM aggregated_neighbours
        ) AS buildings_with_neighbours
    );