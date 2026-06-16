package config

import (
	"runtime"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	// When
	cfg := DefaultRetryConfig()

	// Then
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 200*time.Millisecond {
		t.Errorf("RetryDelay: got %v, want 200ms", cfg.RetryDelay)
	}
	if cfg.DeadlockRetries != 5 {
		t.Errorf("DeadlockRetries: got %d, want 5", cfg.DeadlockRetries)
	}
	if cfg.DeadlockDelay != 100*time.Millisecond {
		t.Errorf("DeadlockDelay: got %v, want 100ms", cfg.DeadlockDelay)
	}
}

func TestGetBuildingLimit(t *testing.T) {
	cases := []struct {
		name   string
		envVal string
		want   int
	}{
		{"empty returns 0", "", 0},
		{"valid positive", "100", 100},
		{"zero returns 0", "0", 0},
		{"negative is rejected", "-5", 0},
		{"invalid string is rejected", "abc", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			t.Setenv("BUILDING_LIMIT", tc.envVal)

			// When
			got := getBuildingLimit()

			// Then
			if got != tc.want {
				t.Errorf("getBuildingLimit() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGetThreadCount(t *testing.T) {
	numCPU := max(runtime.NumCPU(), 1)

	cases := []struct {
		name   string
		envVal string
		want   int
	}{
		{"empty uses NumCPU", "", numCPU},
		{"valid value within range", "1", 1},
		{"zero clamps to 1", "0", 1},
		{"negative clamps to 1", "-2", 1},
		{"above NumCPU clamps to NumCPU", "99999", numCPU},
		{"invalid string falls back to NumCPU", "abc", numCPU},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			t.Setenv("THREAD_COUNT", tc.envVal)

			// When
			got := getThreadCount()

			// Then
			if got != tc.want {
				t.Errorf("getThreadCount() = %d, want %d", got, tc.want)
			}
		})
	}
}
