package config

import "strconv"

// City2TabulaConfig holds City2TABULA specific configuration
type City2TabulaConfig struct {
	RoomHeight   string // Default room height in meters used for volume calculations
	LinkGridSize int    // Grid cell side length in metres for PyLovo spatial batching (PYLOVO_LINK_GRID_SIZE)
}

// loadCity2TabulaConfig loads City2TABULA specific configuration
func loadCity2TabulaConfig() *City2TabulaConfig {
	roomHeight := GetEnv("ROOM_HEIGHT", "2.5")

	gridSize, err := strconv.Atoi(GetEnv("PYLOVO_LINK_GRID_SIZE", "1000"))
	if err != nil || gridSize <= 0 {
		gridSize = 1000 // default: 1 km grid cells
	}

	return &City2TabulaConfig{
		RoomHeight:   roomHeight,
		LinkGridSize: gridSize,
	}
}
