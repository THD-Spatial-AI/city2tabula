package utils

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/THD-Spatial/City2TABULA/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExecuteCommand executes a shell command and returns an error if it fails
func ExecuteCommand(command string) error {
	Info.Printf("Executing command: %s", command)
	unixCommand := "sh"
	windowsCommand := "cmd"
	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.Command(windowsCommand, "/C", command)
	} else {
		cmd = exec.Command(unixCommand, "-c", command)
	}

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	if err != nil {
		Error.Printf("Command failed: %s", string(output))
		return fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}

	Info.Printf("Command output: %s", string(output))
	return nil
}

func isWindows() bool {
	return strings.Contains(strings.ToLower(runtime.GOOS), "windows")
}

// parseSRID parses the CityDB CRS string and returns the SRID
func parseSRID(crs string) (int, error) {
	// Accept "EPSG:25832" or plain "25832"
	c := strings.TrimSpace(strings.ToUpper(crs))
	c = strings.TrimPrefix(c, "EPSG:")
	srid, err := strconv.Atoi(c)
	if err != nil || srid <= 0 {
		return 0, fmt.Errorf("invalid CityDB CRS '%s' (expect EPSG:XXXX or XXXX)", crs)
	}
	return srid, nil
}

func ExecuteSQLScript(sqlScript string, config *config.Config, conn *pgxpool.Pool, lod int, buildingIDs []int64) error {
	// Add detailed error logging with stack trace info
	if config == nil {
		Error.Printf("ExecuteSQLScript called with nil config")
		Error.Printf("sqlScript length: %d", len(sqlScript))
		Error.Printf("conn is nil: %v", conn == nil)
		Error.Printf("lod: %d", lod)
		Error.Printf("buildingIDs: %v", buildingIDs)
		return fmt.Errorf("config parameter cannot be nil - check the calling function")
	}
	// Get all available parameters
	sqlParams := config.GetSQLParameters(lod, buildingIDs)
	params := make(map[string]any)

	// Use reflection to dynamically extract parameters
	paramMap := getSQLParameterMap(sqlParams)

	// Include all available parameters
	for key, value := range paramMap {
		// Special handling for building_ids
		if key == "building_ids" && value != nil {
			if ids, ok := value.([]int64); ok {
				// Only format if there are building IDs
				if len(ids) > 0 {
					// Format as "(1,2,3)" for SQL
					idStrings := make([]string, len(ids))
					for i, id := range ids {
						idStrings[i] = fmt.Sprintf("%d", id)
					}
					params[key] = fmt.Sprintf("(%s)", strings.Join(idStrings, ","))
				} else {
					// Empty slice - use (-1) to prevent syntax error while matching nothing
					params[key] = "(-1)"
				}
			} else {
				return fmt.Errorf("building_ids parameter is not of type []int64")
			}
		} else {
			params[key] = value
		}
	}

	replacedScript, err := replaceParameters(sqlScript, params)
	if err != nil {
		return err
	}

	// // Execute with timeout to prevent hanging
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()
	ctx := context.Background()
	if _, err := conn.Exec(ctx, replacedScript); err != nil {
		return err
	}
	return nil
}

// getSQLParameterMap uses reflection to extract parameter mappings
func getSQLParameterMap(params config.SQLParameters) map[string]any {
	paramMap := make(map[string]any)

	v := reflect.ValueOf(params)
	t := reflect.TypeOf(params)

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		paramTag := field.Tag.Get("param")
		if paramTag != "" {
			paramMap[paramTag] = v.Field(i).Interface()
		}
	}

	return paramMap
}

// replaceParameters replaces placeholders in the SQL script with actual values
func replaceParameters(sqlScript string, params map[string]any) (string, error) {
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		sqlScript = strings.ReplaceAll(sqlScript, placeholder, fmt.Sprintf("%v", value))
	}
	return sqlScript, nil
}
