package utils

import (
	"City2TABULA/internal/config"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/jackc/pgx/v5/pgxpool"
)

func GetBuildingFeatureCount(dbConn *pgxpool.Pool, schemaName string) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.feature WHERE objectclass_id IN (901, 905)`, schemaName)
	var count int
	err := dbConn.QueryRow(context.Background(), query).Scan(&count)
	return count, err
}

// GetBuildingIDsFromCityDB fetches building feature IDs from specified LOD schema
func GetBuildingIDsFromCityDB(dbConn *pgxpool.Pool, schemaName string) ([]int64, error) {
	// Query to get building feature IDs (objectclass_id 901 for building feature and 905 for building part feature)
	query := fmt.Sprintf(`
		SELECT f.id
		FROM %s.feature f
		WHERE f.objectclass_id IN (901, 905)
		ORDER BY f.id`, schemaName)

	rows, err := dbConn.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query building IDs from %s: %w", schemaName, err)
	}
	defer rows.Close()

	var buildingIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan building ID: %w", err)
		}
		buildingIDs = append(buildingIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return buildingIDs, nil
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
