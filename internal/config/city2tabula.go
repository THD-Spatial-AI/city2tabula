package config

import "strconv"

// City2TabulaConfig holds City2TABULA specific configuration
type City2TabulaConfig struct {
	StoreyHeight string // Floor-to-floor height in metres for counting storeys (STOREY_HEIGHT)
	LinkGridSize int    // Grid cell side length in metres for PyLovo spatial batching (PYLOVO_LINK_GRID_SIZE)
}

// loadCity2TabulaConfig loads City2TABULA specific configuration
func loadCity2TabulaConfig() *City2TabulaConfig {
	storeyHeight := GetEnv("STOREY_HEIGHT", "2.8")

	gridSize, err := strconv.Atoi(GetEnv("PYLOVO_LINK_GRID_SIZE", "1000"))
	if err != nil || gridSize <= 0 {
		gridSize = 1000 // default: 1 km grid cells
	}

	return &City2TabulaConfig{
		StoreyHeight: storeyHeight,
		LinkGridSize: gridSize,
	}
}
