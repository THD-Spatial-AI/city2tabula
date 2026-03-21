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

// ExecuteSQLScript substitutes template parameters into sqlScript and executes it.
// Parameters use {param_name} placeholders — see config.SQLParameters for the full list.
// lod should be 2 or 3 for feature extraction, -1 for non-LOD tasks (schema setup, functions, etc.).
func ExecuteSQLScript(sqlScript string, cfg *config.Config, conn *pgxpool.Pool, lod int, buildingIDs []int64) error {
	if cfg == nil {
		return fmt.Errorf("ExecuteSQLScript: config cannot be nil")
	}

	params := buildParamMap(cfg.GetSQLParameters(lod, buildingIDs))

	replacedScript, err := replaceParameters(sqlScript, params)
	if err != nil {
		return err
	}

	if _, err := conn.Exec(context.Background(), replacedScript); err != nil {
		return err
	}
	return nil
}

// buildParamMap converts an SQLParameters struct into a map ready for placeholder substitution.
// Building IDs get special treatment: they're formatted as a SQL tuple "(1,2,3)" for use in IN clauses.
// An empty slice becomes "(-1)" — a safe value that matches nothing and avoids a syntax error.
func buildParamMap(sqlParams config.SQLParameters) map[string]any {
	params := getSQLParameterMap(sqlParams)
	if ids, ok := params["building_ids"].([]int64); ok {
		params["building_ids"] = formatBuildingIDs(ids)
	}
	return params
}

// formatBuildingIDs formats a slice of building IDs as a SQL tuple: "(1,2,3)".
// Returns "(-1)" for an empty slice so IN clauses remain valid SQL but match nothing.
func formatBuildingIDs(ids []int64) string {
	if len(ids) == 0 {
		return "(-1)"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return "(" + strings.Join(parts, ",") + ")"
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
