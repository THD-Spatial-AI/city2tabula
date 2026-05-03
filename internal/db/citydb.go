package db

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

func executeCityDBScript(cfg *config.Config, sqlFilePath, schemaName string) error {
	utils.Debug.Printf("Executing CityDB script: %s", sqlFilePath)

	srid, err := parseSRID(cfg.CityDB.SRID)
	if err != nil {
		return err
	}

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
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.DB.Password)

	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		utils.Debug.Printf("psql output:\n%s", string(out))
	}
	if err != nil {
		if strings.Contains(string(out), "already exists") {
			return fmt.Errorf("schema already exists (psql output: %s): %w", strings.TrimSpace(string(out)), err)
		}
		return fmt.Errorf("failed to execute CityDB script %s: %w", sqlFilePath, err)
	}
	return nil
}

func parseSRID(crs string) (int, error) {
	c := strings.TrimSpace(strings.ToUpper(crs))
	c = strings.TrimPrefix(c, "EPSG:")
	srid, err := strconv.Atoi(c)
	if err != nil || srid <= 0 {
		return 0, fmt.Errorf("invalid CityDB CRS '%s' (expect EPSG:XXXX or XXXX)", crs)
	}
	return srid, nil
}
