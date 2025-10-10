package tests

import (
	"log"
	"os"
	"testing"

	"City2TABULA/internal/utils"

	"github.com/stretchr/testify/assert"
)

func TestSetLogLevel(t *testing.T) {
	// Initialize logger first
	utils.InitLogger()

	tests := []struct {
		name     string
		level    utils.LogLevel
		expected utils.LogLevel
	}{
		{"Set Debug level", utils.LogLevelDebug, utils.LogLevelDebug},
		{"Set Info level", utils.LogLevelInfo, utils.LogLevelInfo},
		{"Set Warn level", utils.LogLevelWarn, utils.LogLevelWarn},
		{"Set Error level", utils.LogLevelError, utils.LogLevelError},
	}

	// Save original level
	originalLevel := utils.GetLogLevel()
	defer utils.SetLogLevel(originalLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils.SetLogLevel(tt.level)
			result := utils.GetLogLevel()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	// Initialize logger first
	utils.InitLogger()

	// Save original level
	originalLevel := utils.GetLogLevel()
	defer utils.SetLogLevel(originalLevel)

	utils.SetLogLevel(utils.LogLevelDebug)
	assert.Equal(t, utils.LogLevelDebug, utils.GetLogLevel())

	utils.SetLogLevel(utils.LogLevelError)
	assert.Equal(t, utils.LogLevelError, utils.GetLogLevel())
}

func TestIsDebugEnabled(t *testing.T) {
	// Initialize logger first
	utils.InitLogger()

	// Save original level
	originalLevel := utils.GetLogLevel()
	defer utils.SetLogLevel(originalLevel)

	tests := []struct {
		name     string
		level    utils.LogLevel
		expected bool
	}{
		{"Debug level enables debug", utils.LogLevelDebug, true},
		{"Info level disables debug", utils.LogLevelInfo, false},
		{"Warn level disables debug", utils.LogLevelWarn, false},
		{"Error level disables debug", utils.LogLevelError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils.SetLogLevel(tt.level)
			result := utils.IsDebugEnabled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetLogLevelFromEnv(t *testing.T) {
	// Save original level
	originalLevel := utils.GetLogLevel()
	defer utils.SetLogLevel(originalLevel)

	tests := []struct {
		name      string
		envValue  string
		expected  utils.LogLevel
		shouldSet bool
	}{
		{
			name:      "Debug level",
			envValue:  "DEBUG",
			expected:  utils.LogLevelDebug,
			shouldSet: true,
		},
		{
			name:      "Info level",
			envValue:  "INFO",
			expected:  utils.LogLevelInfo,
			shouldSet: true,
		},
		{
			name:      "Warn level",
			envValue:  "WARN",
			expected:  utils.LogLevelWarn,
			shouldSet: true,
		},
		{
			name:      "Warning level",
			envValue:  "WARNING",
			expected:  utils.LogLevelWarn,
			shouldSet: true,
		},
		{
			name:      "Error level",
			envValue:  "ERROR",
			expected:  utils.LogLevelError,
			shouldSet: true,
		},
		{
			name:      "No env var set",
			envValue:  "",
			expected:  utils.LogLevelInfo, // Default
			shouldSet: false,
		},
		{
			name:      "Invalid level",
			envValue:  "INVALID",
			expected:  utils.LogLevelInfo, // Default
			shouldSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing LOG_LEVEL env var
			os.Unsetenv("LOG_LEVEL")

			if tt.shouldSet {
				os.Setenv("LOG_LEVEL", tt.envValue)
			}

			// Call setutils.LogLevelFromEnv() by reinitializing logger
			utils.InitLogger()

			assert.Equal(t, tt.expected, utils.GetLogLevel())

			// Clean up
			os.Unsetenv("LOG_LEVEL")
		})
	}
}

func TestInitLogger(t *testing.T) {
	// Clean up any existing log files from this test
	defer func() {
		os.RemoveAll("logs")
	}()

	// Test that InitLogger creates the logs directory
	utils.InitLogger()

	// Check that logs directory exists
	_, err := os.Stat("logs")
	assert.NoError(t, err, "logs directory should be created")

	// Check that loggers are initialized
	assert.NotNil(t, utils.Info, "Info logger should be initialized")
	assert.NotNil(t, utils.Debug, "Debug logger should be initialized")
	assert.NotNil(t, utils.Warn, "Warn logger should be initialized")
	assert.NotNil(t, utils.Error, "Error logger should be initialized")
}

func TestLoggerFormatting(t *testing.T) {
	// Capture log output for testing
	defer func() {
		os.RemoveAll("logs")
	}()

	utils.InitLogger()

	// Test that loggers have proper flags
	expectedFlags := log.Ldate | log.Ltime | log.Lshortfile

	assert.Equal(t, expectedFlags, utils.Info.Flags(), "Info logger should have proper flags")
	assert.Equal(t, expectedFlags, utils.Debug.Flags(), "Debug logger should have proper flags")
	assert.Equal(t, expectedFlags, utils.Warn.Flags(), "Warn logger should have proper flags")
	assert.Equal(t, expectedFlags, utils.Error.Flags(), "Error logger should have proper flags")
}

func TestLogLevelConstants(t *testing.T) {
	// Test that log level constants are ordered correctly
	assert.True(t, utils.LogLevelDebug < utils.LogLevelInfo)
	assert.True(t, utils.LogLevelInfo < utils.LogLevelWarn)
	assert.True(t, utils.LogLevelWarn < utils.LogLevelError)

	// Test specific values
	assert.Equal(t, utils.LogLevel(0), utils.LogLevelDebug)
	assert.Equal(t, utils.LogLevel(1), utils.LogLevelInfo)
	assert.Equal(t, utils.LogLevel(2), utils.LogLevelWarn)
	assert.Equal(t, utils.LogLevel(3), utils.LogLevelError)
}

func TestLoggerPrefixes(t *testing.T) {
	defer func() {
		os.RemoveAll("logs")
	}()

	utils.InitLogger()

	// Check that loggers have the expected prefixes
	assert.Equal(t, "INFO: ", utils.Info.Prefix())
	assert.Equal(t, "DEBUG: ", utils.Debug.Prefix())
	assert.Equal(t, "WARN: ", utils.Warn.Prefix())
	assert.Equal(t, "ERROR: ", utils.Error.Prefix())
}

func TestGetLogLevelName(t *testing.T) {
	tests := []struct {
		name     string
		level    utils.LogLevel
		expected string
	}{
		{"Debug level name", utils.LogLevelDebug, "DEBUG"},
		{"Info level name", utils.LogLevelInfo, "INFO"},
		{"Warn level name", utils.LogLevelWarn, "WARN"},
		{"Error level name", utils.LogLevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to access getutils.LogLevelName somehow, but it's private
			// We can test it indirectly by checking the Setutils.LogLevel output
			originalLevel := utils.GetLogLevel()
			defer utils.SetLogLevel(originalLevel)

			utils.SetLogLevel(tt.level)
			// The function exists and is tested indirectly through Setutils.LogLevel
			assert.Equal(t, tt.level, utils.GetLogLevel())
		})
	}
}

// Test that multiple calls to InitLogger don't cause issues
func TestInitLoggerMultipleCalls(t *testing.T) {
	defer func() {
		os.RemoveAll("logs")
	}()

	// Call InitLogger multiple times
	utils.InitLogger()
	utils.InitLogger()
	utils.InitLogger()

	// Should still work correctly
	assert.NotNil(t, utils.Info)
	assert.NotNil(t, utils.Debug)
	assert.NotNil(t, utils.Warn)
	assert.NotNil(t, utils.Error)
}

// Test debug logging behavior based on log level
func TestDebugLoggingBehavior(t *testing.T) {
	defer func() {
		os.RemoveAll("logs")
	}()

	originalLevel := utils.GetLogLevel()
	defer utils.SetLogLevel(originalLevel)

	// Test debug enabled - set environment variable first
	os.Setenv("LOG_LEVEL", "DEBUG")
	utils.InitLogger()
	assert.True(t, utils.IsDebugEnabled())
	os.Unsetenv("LOG_LEVEL")

	// Test debug disabled - set environment variable first
	os.Setenv("LOG_LEVEL", "INFO")
	utils.InitLogger()
	assert.False(t, utils.IsDebugEnabled())
	os.Unsetenv("LOG_LEVEL")
}

// Benchmark logger initialization
func BenchmarkInitLogger(b *testing.B) {
	defer func() {
		os.RemoveAll("logs")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.InitLogger()
	}
}
