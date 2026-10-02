package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// Dataset is one source dataset folder under a LOD data directory. The model
// files sit in Inputs, its subfolders, and are imported into Schema with the
// dataset_id as each feature's lineage.
type Dataset struct {
	Dir         string
	Schema      string
	Inputs      []string
	Attribution attribution.Attribution
}

// modelExtensions are the file types citydb-tool imports. One lying directly in
// a LOD directory belongs to no dataset folder, so it has no attribution.
var modelExtensions = map[string]bool{".gml": true, ".xml": true, ".json": true, ".jsonl": true}

// DiscoverDatasets returns every dataset folder under cfg's LOD2 and LOD3 data
// directories with its validated attribution. A missing LOD directory is
// skipped, which keeps LOD3 optional. A model file directly in a LOD directory
// or a dataset folder, a folder without a valid attribution.json, or a
// dataset_id claimed by two folders is an error naming the path.
func DiscoverDatasets(cfg *config.Config) ([]Dataset, error) {
	var datasets []Dataset
	for _, lod := range []struct{ dir, schema string }{
		{cfg.Data.Lod2, cfg.DB.Schemas.Lod2},
		{cfg.Data.Lod3, cfg.DB.Schemas.Lod3},
	} {
		found, err := discoverLOD(lod.dir, lod.schema)
		if err != nil {
			return nil, err
		}
		datasets = append(datasets, found...)
	}

	seen := map[string]string{}
	for _, d := range datasets {
		if other, ok := seen[d.Attribution.DatasetID]; ok {
			return nil, fmt.Errorf("dataset_id %q is used by both %s and %s", d.Attribution.DatasetID, other, d.Dir)
		}
		seen[d.Attribution.DatasetID] = d.Dir
	}
	return datasets, nil
}

// discoverLOD returns the dataset folders directly under dir. Hidden entries
// and non-model files such as a README are ignored.
func discoverLOD(dir, schema string) ([]Dataset, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		utils.Warn.Printf("Data path not found: %s, skipping", dir)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read data directory %s: %w", dir, err)
	}

	var datasets []Dataset
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if !e.IsDir() {
			if modelExtensions[strings.ToLower(filepath.Ext(e.Name()))] {
				return nil, fmt.Errorf("%s lies directly in %s; move it into a dataset folder holding an %s", path, dir, attribution.FileName)
			}
			continue
		}
		a, err := attribution.Load(path)
		if err != nil {
			return nil, err
		}
		inputs, err := datasetInputs(path)
		if err != nil {
			return nil, err
		}
		datasets = append(datasets, Dataset{Dir: path, Schema: schema, Inputs: inputs, Attribution: a})
	}
	return datasets, nil
}

// datasetInputs returns a dataset folder's subfolders, which hold its model
// files. citydb-tool would read attribution.json as CityJSON if given the folder,
// and imports an explicitly named file whatever its format.
func datasetInputs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dataset folder %s: %w", dir, err)
	}
	var inputs []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), "."), e.Name() == attribution.FileName:
		case e.IsDir():
			inputs = append(inputs, path)
		case modelExtensions[strings.ToLower(filepath.Ext(e.Name()))]:
			return nil, fmt.Errorf("%s lies directly in dataset folder %s; move it into a subfolder such as %s", path, dir, filepath.Join(dir, "gml"))
		}
	}
	if len(inputs) == 0 {
		utils.Warn.Printf("Dataset folder %s has no subfolders with model files, nothing to import from it", dir)
	}
	return inputs, nil
}

// SyncAttribution reads the attribution file of every 3D dataset folder and of
// the TABULA data, and upserts one dataset_attribution row per dataset. It
// imports nothing, so correcting a credit needs no re-import. Every file is
// validated before any row is written. Returns how many rows were inserted or
// changed.
func SyncAttribution(ctx context.Context, conn *pgxpool.Pool, cfg *config.Config) (int64, error) {
	datasets, err := DiscoverDatasets(cfg)
	if err != nil {
		return 0, err
	}
	tabula, err := attribution.Load(cfg.Data.Tabula)
	if err != nil {
		return 0, err
	}
	datasets = append(datasets, Dataset{Dir: cfg.Data.Tabula, Attribution: tabula})

	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin attribution sync: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	var changed int64
	for _, d := range datasets {
		n, err := upsertAttribution(ctx, tx, cfg.DB.Schemas.City2Tabula, d)
		if err != nil {
			return 0, err
		}
		changed += n
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit attribution sync: %w", err)
	}
	return changed, nil
}

// upsertAttribution writes d's row. An unchanged row is left alone, so
// updated_at records when the credit last changed, not when it was last read.
func upsertAttribution(ctx context.Context, tx pgx.Tx, schema string, d Dataset) (int64, error) {
	a := d.Attribution
	q := fmt.Sprintf(`
		INSERT INTO %[1]s.dataset_attribution AS t
			(dataset_id, provider, dataset, licence, licence_url, credit, credit_url, terms_url, changes, source_path)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10)
		ON CONFLICT (dataset_id) DO UPDATE SET
			provider = EXCLUDED.provider, dataset = EXCLUDED.dataset, licence = EXCLUDED.licence,
			licence_url = EXCLUDED.licence_url, credit = EXCLUDED.credit, credit_url = EXCLUDED.credit_url,
			terms_url = EXCLUDED.terms_url, changes = EXCLUDED.changes, source_path = EXCLUDED.source_path,
			updated_at = NOW()
		WHERE (t.provider, t.dataset, t.licence, t.licence_url, t.credit, t.credit_url, t.terms_url, t.changes, t.source_path)
			IS DISTINCT FROM
			(EXCLUDED.provider, EXCLUDED.dataset, EXCLUDED.licence, EXCLUDED.licence_url, EXCLUDED.credit,
			 EXCLUDED.credit_url, EXCLUDED.terms_url, EXCLUDED.changes, EXCLUDED.source_path)`,
		pgx.Identifier{schema}.Sanitize())

	tag, err := tx.Exec(ctx, q, a.DatasetID, a.Provider, a.Dataset, a.Licence, a.LicenceURL,
		a.Credit, a.CreditURL, a.TermsURL, a.Changes, d.Dir)
	if err != nil {
		return 0, fmt.Errorf("upsert attribution for %s (%s): %w", a.DatasetID, d.Dir, err)
	}
	return tag.RowsAffected(), nil
}
