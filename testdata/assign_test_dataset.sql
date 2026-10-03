-- Gives every seeded 3DCityDB feature the lineage an import from a dataset
-- folder would set, and writes the dataset_attribution rows an import writes:
-- that dataset's and TABULA's. Seed files stand in for the import in tests.
-- Safe to run more than once.
INSERT INTO city2tabula.dataset_attribution
    (dataset_id, provider, dataset, licence, licence_url, credit, changes, source_path)
VALUES
    ('test-dataset', 'Test provider', 'Test buildings', 'CC-BY-4.0',
     'https://creativecommons.org/licenses/by/4.0/', 'Test provider', 'Test fixture', 'testdata'),
    ('tabula-episcope', 'IEE Projects TABULA + EPISCOPE', 'TABULA building typologies',
     'LicenseRef-TABULA-EPISCOPE', 'https://episcope.eu/communication/download/',
     'IEE Projects TABULA + EPISCOPE (www.episcope.eu)', 'Test fixture', 'data/tabula')
ON CONFLICT (dataset_id) DO NOTHING;

DO $$
BEGIN
    IF to_regclass('lod2.feature') IS NOT NULL THEN
        UPDATE lod2.feature SET lineage = 'test-dataset' WHERE lineage IS NULL;
    END IF;
    IF to_regclass('lod3.feature') IS NOT NULL THEN
        UPDATE lod3.feature SET lineage = 'test-dataset' WHERE lineage IS NULL;
    END IF;
END $$;
