package importer

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/attribution"
	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// BuildingUseFile is the file in an attribute dataset folder that lists the uses
// of each building: one row per use code, with the floor area it covers.
const BuildingUseFile = "building_use.csv"

// buildingUseHeader is the first row BuildingUseFile must carry.
var buildingUseHeader = []string{"object_id", "use_code", "floor_area"}

// buildingUseRow is one parsed row of BuildingUseFile.
type buildingUseRow struct {
	objectID, useCode string
	floorArea         float64
}

// DiscoverAttributeDatasets returns every dataset folder under cfg's attributes
// directory with its validated attribution. A missing directory is skipped, as
// most countries have no attribute data. A folder without a valid
// attribution.json or without BuildingUseFile is an error naming the path.
func DiscoverAttributeDatasets(cfg *config.Config) ([]Dataset, error) {
	dir := cfg.Data.Attributes
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read attributes directory %s: %w", dir, err)
	}

	var datasets []Dataset
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		a, err := attribution.Load(path)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(path, BuildingUseFile)); err != nil {
			return nil, fmt.Errorf("attribute dataset folder %s has no %s: %w", path, BuildingUseFile, err)
		}
		datasets = append(datasets, Dataset{Dir: path, Attribution: a})
	}
	return datasets, nil
}

// ImportBuildingUse replaces each attribute dataset's rows in the building_use
// table with the rows of its BuildingUseFile. Every file is parsed before any
// row is written, so a malformed file changes nothing.
func ImportBuildingUse(ctx context.Context, conn *pgxpool.Pool, cfg *config.Config) error {
	datasets, err := DiscoverAttributeDatasets(cfg)
	if err != nil || len(datasets) == 0 {
		return err
	}
	rows := make([][]buildingUseRow, len(datasets))
	for i, d := range datasets {
		if rows[i], err = readBuildingUse(filepath.Join(d.Dir, BuildingUseFile)); err != nil {
			return err
		}
	}

	table := pgx.Identifier{cfg.DB.Schemas.City2Tabula, "building_use"}
	if err := ensureBuildingUseTable(ctx, conn, table); err != nil {
		return err
	}
	for i, d := range datasets {
		if err := replaceBuildingUse(ctx, conn, table, d.Attribution.DatasetID, rows[i]); err != nil {
			return fmt.Errorf("import %s: %w", filepath.Join(d.Dir, BuildingUseFile), err)
		}
		utils.Info.Printf("Imported %d building use rows from %s", len(rows[i]), d.Dir)
	}
	return nil
}

// ensureBuildingUseTable creates the building_use table if it is missing. It is
// created here rather than by the schema setup, so a database built by an
// earlier release gains it on its next import.
func ensureBuildingUseTable(ctx context.Context, conn *pgxpool.Pool, table pgx.Identifier) error {
	name := table.Sanitize()
	stmt := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %[1]s (
			dataset_id TEXT             NOT NULL,
			object_id  TEXT             NOT NULL,
			use_code   TEXT             NOT NULL,
			floor_area DOUBLE PRECISION NOT NULL CHECK (floor_area >= 0)
		);
		CREATE INDEX IF NOT EXISTS building_use_object_id_idx ON %[1]s (object_id);`, name)
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	return nil
}

// replaceBuildingUse deletes datasetID's rows and copies rows in, in one
// transaction.
func replaceBuildingUse(ctx context.Context, conn *pgxpool.Pool, table pgx.Identifier, datasetID string, rows []buildingUseRow) error {
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM `+table.Sanitize()+` WHERE dataset_id = $1`, datasetID); err != nil {
			return err
		}
		_, err := tx.CopyFrom(ctx, table, []string{"dataset_id", "object_id", "use_code", "floor_area"},
			pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
				return []any{datasetID, rows[i].objectID, rows[i].useCode, rows[i].floorArea}, nil
			}))
		return err
	})
}

// readBuildingUse parses a BuildingUseFile. Every error names the file and line.
func readBuildingUse(path string) ([]buildingUseRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseBuildingUse(f, path)
}

func parseBuildingUse(r io.Reader, path string) ([]buildingUseRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = len(buildingUseHeader)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: read header: %w", path, err)
	}
	if !slices.Equal(header, buildingUseHeader) {
		return nil, fmt.Errorf("%s: header is %q, want %q", path, header, buildingUseHeader)
	}

	var rows []buildingUseRow
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		line, _ := cr.FieldPos(0)
		row, err := parseBuildingUseRow(rec)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		rows = append(rows, row)
	}
}

func parseBuildingUseRow(rec []string) (buildingUseRow, error) {
	if rec[0] == "" || rec[1] == "" {
		return buildingUseRow{}, fmt.Errorf("object_id and use_code are required, got %q", rec)
	}
	if strings.Contains(rec[1], ",") {
		return buildingUseRow{}, fmt.Errorf("use_code %q holds several codes; give each its own row", rec[1])
	}
	area, err := strconv.ParseFloat(rec[2], 64)
	if err != nil || area < 0 || math.IsInf(area, 0) || math.IsNaN(area) {
		return buildingUseRow{}, fmt.Errorf("floor_area %q is not a number >= 0", rec[2])
	}
	return buildingUseRow{objectID: rec[0], useCode: rec[1], floorArea: area}, nil
}
