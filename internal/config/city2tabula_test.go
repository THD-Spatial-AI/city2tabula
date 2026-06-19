package config

import "testing"

func TestLoadCity2TabulaConfig_LinkGridSize(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   int
	}{
		{"default when empty", "", 1000},
		{"valid value", "500", 500},
		{"invalid string falls back to default", "abc", 1000},
		{"zero falls back to default", "0", 1000},
		{"negative falls back to default", "-100", 1000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			t.Setenv("PYLOVO_LINK_GRID_SIZE", tc.envVal)

			// When
			cfg := loadCity2TabulaConfig()

			// Then
			if cfg.LinkGridSize != tc.want {
				t.Errorf("LinkGridSize = %d, want %d", cfg.LinkGridSize, tc.want)
			}
		})
	}
}
