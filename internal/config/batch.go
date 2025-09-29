package config

import (
	"runtime"
	"strconv"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	DeadlockRetries int
}

// BatchConfig holds batch processing configuration
type BatchConfig struct {
	Size           int
	Threads        int
	DBMaxOpenConns int
	DBMaxIdleConns int
}

// DefaultRetryConfig returns sensible defaults for retry behaviour
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffFactor:   2.0,
		DeadlockRetries: 5,
	}
}

// loadBatchConfig loads batch processing configuration
func loadBatchConfig() *BatchConfig {
	return &BatchConfig{
		Size:           1000,
		Threads:        getThreadCount(),
		DBMaxOpenConns: GetEnvAsInt("DB_MAX_OPEN_CONNS", 10),
		DBMaxIdleConns: GetEnvAsInt("DB_MAX_IDLE_CONNS", 5),
	}
}

// getThreadCount determines optimal thread count
func getThreadCount() int {
	numCPU := max(runtime.NumCPU(), 1)

	if envThreads := GetEnv("THREAD_COUNT", ""); envThreads != "" {
		if t, err := strconv.Atoi(envThreads); err == nil {
			if t < 1 {
				// Could use logger here
				return 1
			}
			if t > numCPU {
				// Could use logger here
				return numCPU
			}
			return t
		}
		// Could use logger here for invalid thread count
	}

	return numCPU
}
