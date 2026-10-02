-- One row per source dataset, read from the dataset folder's attribution.json
-- (internal/attribution). Every {lod_schema}_building row references its dataset
-- here, so no building exists without a credit.
--
-- Created by the supplementary setup, which runs once before the per-LOD main
-- setup jobs: both LOD building tables hold a foreign key to this table, and a
-- per-LOD DROP ... CASCADE would remove the other LOD's constraint.
DROP TABLE IF EXISTS {city2tabula_schema}.dataset_attribution CASCADE;
CREATE TABLE {city2tabula_schema}.dataset_attribution (
    dataset_id  TEXT        PRIMARY KEY,
    provider    TEXT        NOT NULL,
    dataset     TEXT        NOT NULL,
    licence     TEXT        NOT NULL,
    licence_url TEXT        NOT NULL,
    credit      TEXT        NOT NULL,
    credit_url  TEXT,
    terms_url   TEXT,
    changes     TEXT        NOT NULL,
    -- Folder the row was last read from, relative to the working directory.
    source_path TEXT        NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
