package process

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

func executeSQLScript(sqlScript string, cfg *config.Config, conn *pgxpool.Pool, lod int, buildingIDs []int64) error {
	if cfg == nil {
		return fmt.Errorf("config parameter cannot be nil")
	}

	sqlParams := cfg.GetSQLParameters(lod, buildingIDs)
	paramMap := getSQLParameterMap(sqlParams)
	params := make(map[string]any)

	for key, value := range paramMap {
		if key == "building_ids" && value != nil {
			if ids, ok := value.([]int64); ok {
				if len(ids) > 0 {
					idStrings := make([]string, len(ids))
					for i, id := range ids {
						idStrings[i] = fmt.Sprintf("%d", id)
					}
					params[key] = fmt.Sprintf("(%s)", strings.Join(idStrings, ","))
				} else {
					params[key] = "(-1)"
				}
			} else {
				return fmt.Errorf("building_ids parameter is not of type []int64")
			}
		} else {
			params[key] = value
		}
	}

	replacedScript := replaceParameters(sqlScript, params)

	if _, err := conn.Exec(context.Background(), replacedScript); err != nil {
		return err
	}
	return nil
}

func getSQLParameterMap(params config.SQLParameters) map[string]any {
	paramMap := make(map[string]any)
	v := reflect.ValueOf(params)
	t := reflect.TypeOf(params)
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if tag := field.Tag.Get("param"); tag != "" {
			paramMap[tag] = v.Field(i).Interface()
		}
	}
	return paramMap
}

// Finds the keys provided in map with corresponding value in input string
func replaceParameters(sqlScript string, params map[string]any) string {
	for key, value := range params {
		sqlScript = strings.ReplaceAll(sqlScript, fmt.Sprintf("{%s}", key), fmt.Sprintf("%v", value))
	}
	return sqlScript
}
