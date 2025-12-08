package utils

import (
	"City2TABULA/internal/config"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func GetBuildingObjectClassIDs(dbConn *pgxpool.Pool, schemaName string) ([]int, error) {
	query := fmt.Sprintf(`
        SELECT DISTINCT objectclass_id
        FROM %s.feature
        WHERE objectclass_id BETWEEN 900 AND 999
        ORDER BY objectclass_id`, schemaName)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building objectclass_ids: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// GetBuildingIDsFromCityDB fetches building feature IDs from specified LOD schema
func GetBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error) {
	// Fetch all building-related object classes dynamically
	buildingClasses, err := GetBuildingObjectClassIDs(dbConn, schemaName)
	if err != nil {
		return nil, err
	}

	// Return empty slice if no building classes found
	if len(buildingClasses) == 0 {
		return []int64{}, nil
	}

	// Convert slice to comma-separated list
	classList := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(buildingClasses)), ","), "[]")

	// Query all building feature IDs using the dynamic classes
	query := fmt.Sprintf(`
        SELECT id
        FROM %s.feature
        WHERE objectclass_id IN (%s)
        ORDER BY id`, schemaName, classList)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func ExecuteCityDBScript(cfg *config.Config, sqlFilePath, schemaName string) error {
	Info.Printf("Executing CityDB script: %s", sqlFilePath)

	srid, err := parseSRID(cfg.CityDB.SRID)
	if err != nil {
		return err
	}

	// psql args
	args := []string{
		"-h", cfg.DB.Host,
		"-U", cfg.DB.User,
		"-d", cfg.DB.Name,
		"-p", cfg.DB.Port,
		"-v", fmt.Sprintf("srid=%d", srid),
		"-v", fmt.Sprintf("srs_name=%s", cfg.CityDB.SRSName),
		"-f", sqlFilePath,
	}
	if schemaName != "" {
		args = append([]string{"-v", fmt.Sprintf("schema_name=%s", schemaName)}, args...)
	}

	cmd := exec.Command("psql", args...)
	// Pass password via env; avoid in command line
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.DB.Password)

	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		Info.Printf("psql output:\n%s", string(out))
	}
	return err
}
