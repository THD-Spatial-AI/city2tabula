package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from .env file
func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		// Could use a logger here when available
		// For now, silently continue if no .env file
	}
}

// GetEnv returns environment variable or fallback
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetEnvAsInt returns environment variable as integer or fallback
func GetEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

// normalizeCountryName normalizes country names for consistency
func normalizeCountryName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	missing := []string{}

	if strings.TrimSpace(c.DB.Host) == "" {
		missing = append(missing, "DB_HOST")
	}
	if strings.TrimSpace(c.DB.Port) == "" {
		missing = append(missing, "DB_PORT")
	}
	if strings.TrimSpace(c.DB.User) == "" {
		missing = append(missing, "DB_USER")
	}
	if strings.TrimSpace(c.DB.Password) == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if strings.TrimSpace(c.CityDB.ToolPath) == "" {
		missing = append(missing, "CITYDB_TOOL_PATH")
	}
	if strings.TrimSpace(c.CityDB.SRID) == "" {
		missing = append(missing, "CITYDB_SRID")
	}
	if strings.TrimSpace(c.CityDB.SRSName) == "" {
		missing = append(missing, "CITYDB_SRS_NAME")
	}
	if strings.TrimSpace(c.Country) == "" {
		missing = append(missing, "COUNTRY")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
