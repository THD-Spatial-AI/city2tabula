package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// extentTable records, per dataset, the bboxes already imported from its
// current source files. It lives in the dataset's LOD schema so that dropping
// the imported data also drops the record of it.
const extentTable = "c2t_import_extent"

// extentTimeout bounds each extent query. The citydb import between the check
// and the record can take minutes, so each query gets its own deadline.
const extentTimeout = 30 * time.Second

// importBbox is citydb-tool's --bbox value (xmin,ymin,xmax,ymax[,srid]) parsed.
type importBbox struct {
	xmin, ymin, xmax, ymax float64
	srid                   int
}

// parseImportBbox parses a --bbox value. Without an SRID citydb-tool reads the
// coordinates in the database's SRID, so defaultSRID is used.
func parseImportBbox(s string, defaultSRID string) (importBbox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 && len(parts) != 5 {
		return importBbox{}, fmt.Errorf("bbox %q: want xmin,ymin,xmax,ymax[,srid]", s)
	}
	var nums [4]float64
	for i := range nums {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			return importBbox{}, fmt.Errorf("bbox %q: %w", s, err)
		}
		nums[i] = v
	}
	sridText := defaultSRID
	if len(parts) == 5 {
		sridText = parts[4]
	}
	srid, err := strconv.Atoi(strings.TrimSpace(sridText))
	if err != nil {
		return importBbox{}, fmt.Errorf("bbox %q: srid: %w", s, err)
	}
	return importBbox{nums[0], nums[1], nums[2], nums[3], srid}, nil
}

// inputFingerprint hashes the path, size and modification time of every file
// in the dataset's model folders, so adding, replacing or removing a file
// changes it.
func inputFingerprint(d Dataset) (string, error) {
	h := sha256.New()
	for _, in := range d.Inputs {
		err := filepath.WalkDir(in.path, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			info, err := e.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", in.path, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ensureExtentTable creates the extent table in schema if it is missing. The
// advisory lock serialises runs over disjoint regions, which proceed in
// parallel and would otherwise race on CREATE TABLE IF NOT EXISTS.
func ensureExtentTable(ctx context.Context, conn *pgxpool.Pool, schema string) error {
	table := pgx.Identifier{schema, extentTable}.Sanitize()
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, table); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+table+` (
			dataset_id        text        NOT NULL,
			input_fingerprint text        NOT NULL,
			bbox_mode         text        NOT NULL,
			geom              geometry(Polygon, 4326) NOT NULL,
			imported_at       timestamptz NOT NULL DEFAULT now()
		)`)
		return err
	})
}

// extentImported reports whether an earlier import of d from the same source
// files covered b. An intersects import selects every feature the other modes
// would, so it also covers a contains or on_tile request.
func extentImported(conn *pgxpool.Pool, d Dataset, fingerprint string, b importBbox, mode string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), extentTimeout)
	defer cancel()
	if err := ensureExtentTable(ctx, conn, d.Schema); err != nil {
		return false, fmt.Errorf("create %s.%s: %w", d.Schema, extentTable, err)
	}
	q := `SELECT EXISTS (
		SELECT 1 FROM ` + pgx.Identifier{d.Schema, extentTable}.Sanitize() + `
		WHERE dataset_id = $1 AND input_fingerprint = $2
		  AND (bbox_mode = $3 OR bbox_mode = 'intersects')
		  AND ST_Covers(geom, ST_Transform(ST_MakeEnvelope($4, $5, $6, $7, $8::int), 4326)))`
	var covered bool
	err := conn.QueryRow(ctx, q, d.Attribution.DatasetID, fingerprint, mode,
		b.xmin, b.ymin, b.xmax, b.ymax, b.srid).Scan(&covered)
	if err != nil {
		return false, fmt.Errorf("check imported extent of %s: %w", d.Attribution.DatasetID, err)
	}
	return covered, nil
}

// recordExtent stores b as imported for d, and drops rows recorded against
// source files that have since changed.
func recordExtent(conn *pgxpool.Pool, d Dataset, fingerprint string, b importBbox, mode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), extentTimeout)
	defer cancel()
	table := pgx.Identifier{d.Schema, extentTable}.Sanitize()
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE dataset_id = $1 AND input_fingerprint <> $2`,
			d.Attribution.DatasetID, fingerprint); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO `+table+` (dataset_id, input_fingerprint, bbox_mode, geom)
			VALUES ($1, $2, $3, ST_Transform(ST_MakeEnvelope($4, $5, $6, $7, $8::int), 4326))`,
			d.Attribution.DatasetID, fingerprint, mode, b.xmin, b.ymin, b.xmax, b.ymax, b.srid)
		return err
	})
	if err != nil {
		return fmt.Errorf("record imported extent of %s: %w", d.Attribution.DatasetID, err)
	}
	return nil
}

// importDatasetOnce imports d for bbox unless an earlier import already
// covered it, and records the bbox once the import succeeds. A whole-folder
// import (empty bbox) or a limited one (partial by design) is neither skipped
// nor recorded.
func importDatasetOnce(conn *pgxpool.Pool, cityDBExecPath string, d Dataset, cfg *config.Config, bbox, bboxMode string) error {
	if bbox == "" || cfg.CityDB.ImportLimit > 0 {
		return importCityDBFiles(cityDBExecPath, d, cfg, bbox, bboxMode)
	}
	b, err := parseImportBbox(bbox, cfg.CityDB.SRID)
	if err != nil {
		return err
	}
	fingerprint, err := inputFingerprint(d)
	if err != nil {
		return err
	}

	covered, err := extentImported(conn, d, fingerprint, b, bboxMode)
	if err != nil {
		return err
	}
	if covered {
		utils.Info.Printf("%s already imported for bbox %s, skipping citydb-tool", d.Attribution.DatasetID, bbox)
		return nil
	}

	if err := importCityDBFiles(cityDBExecPath, d, cfg, bbox, bboxMode); err != nil {
		return err
	}
	return recordExtent(conn, d, fingerprint, b, bboxMode)
}
