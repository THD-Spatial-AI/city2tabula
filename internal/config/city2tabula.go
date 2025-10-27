package config

// City2TabulaConfig holds City2TABULA specific configuration
type City2TabulaConfig struct {
	RoomHeight string // Default room height in meters used for volume calculations
}

// loadCity2TabulaConfig loads City2TABULA specific configuration
func loadCity2TabulaConfig() *City2TabulaConfig {
	roomHeight := GetEnv("ROOM_HEIGHT", "2.5") // Default room height in meters

	return &City2TabulaConfig{
		RoomHeight: roomHeight,
	}
}
