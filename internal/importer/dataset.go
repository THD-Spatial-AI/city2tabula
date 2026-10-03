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

// Dataset is one source dataset folder under a LOD data directory. Its model
// folders are imported into Schema with the dataset_id as each feature's lineage.
type Dataset struct {
	Dir         string
	Schema      string
	Inputs      []importInput
	Attribution attribution.Attribution
}

// importInput is one model folder of a dataset and the citydb-tool format that
// reads it.
type importInput struct {
	format, label, path string
}

// modelFolders maps each citydb-tool import format to the dataset subfolder it
// reads. Nothing else in a dataset folder is imported: citydb-tool reads every
// .json in a folder it is given, and archives would be imported as data.
var modelFolders = []struct{ format, label, folder string }{
	{"citygml", "CityGML", "gml"},
	{"cityjson", "CityJSON", "cityjson"},
}

// modelExtensions are the file types citydb-tool imports. One lying directly in
// a LOD directory belongs to no dataset folder, so it has no attribution.
var modelExtensions = map[string]bool{".gml": true, ".xml": true, ".json": true, ".jsonl": true}

// DiscoverDatasets returns every dataset folder under cfg's LOD2 and LOD3 data
// directories with its validated attribution. A missing LOD directory is
// skipped, which keeps LOD3 optional. A model file directly in a LOD directory,
// a folder without a valid attribution.json or without a model folder, or a
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

// datasetInputs returns the model folders of a dataset folder, at least one of
// which must exist. Every other subfolder is logged and left alone.
func datasetInputs(dir string) ([]importInput, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dataset folder %s: %w", dir, err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		present[e.Name()] = true
	}

	var inputs []importInput
	for _, f := range modelFolders {
		if present[f.folder] {
			inputs = append(inputs, importInput{format: f.format, label: f.label, path: filepath.Join(dir, f.folder)})
			delete(present, f.folder)
		}
	}
	for name := range present {
		utils.Info.Printf("Ignoring %s: only gml and cityjson folders are imported", filepath.Join(dir, name))
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("dataset folder %s has neither a gml nor a cityjson subfolder holding its model files", dir)
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
	if tabula.DatasetID != attribution.TabulaDatasetID {
		return 0, fmt.Errorf("%s: dataset_id is %q, want %q",
			filepath.Join(cfg.Data.Tabula, attribution.FileName), tabula.DatasetID, attribution.TabulaDatasetID)
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
// A changed URL clears the row's URL check result, which described the old URL.
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
			updated_at = NOW(),
			url_checked_at = CASE WHEN (t.licence_url, t.credit_url, t.terms_url) IS DISTINCT FROM (EXCLUDED.licence_url, EXCLUDED.credit_url, EXCLUDED.terms_url)
				THEN NULL ELSE t.url_checked_at END,
			url_check_ok = CASE WHEN (t.licence_url, t.credit_url, t.terms_url) IS DISTINCT FROM (EXCLUDED.licence_url, EXCLUDED.credit_url, EXCLUDED.terms_url)
				THEN NULL ELSE t.url_check_ok END,
			url_check_failures = CASE WHEN (t.licence_url, t.credit_url, t.terms_url) IS DISTINCT FROM (EXCLUDED.licence_url, EXCLUDED.credit_url, EXCLUDED.terms_url)
				THEN NULL ELSE t.url_check_failures END
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
