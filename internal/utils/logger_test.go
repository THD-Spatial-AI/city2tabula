package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreLoggerState saves the package-level logger vars and currentLogLevel, and
// returns a func to restore them - these tests mutate global state (Info/Debug/
// Warn/Error, currentLogLevel), which would otherwise leak between tests and into
// any other test in this package that runs afterward in the same process.
func restoreLoggerState(t *testing.T) {
	t.Helper()
	origInfo, origDebug, origWarn, origError := Info, Debug, Warn, Error
	origLevel := currentLogLevel
	t.Cleanup(func() {
		Info, Debug, Warn, Error = origInfo, origDebug, origWarn, origError
		currentLogLevel = origLevel
	})
}

func TestSetLogLevelFromEnv(t *testing.T) {
	restoreLoggerState(t)

	cases := []struct {
		name   string
		envVal string
		unset  bool
		want   LogLevel
	}{
		{name: "DEBUG", envVal: "DEBUG", want: LogLevelDebug},
		{name: "INFO", envVal: "INFO", want: LogLevelInfo},
		{name: "WARN", envVal: "WARN", want: LogLevelWarn},
		{name: "WARNING", envVal: "WARNING", want: LogLevelWarn},
		{name: "ERROR", envVal: "ERROR", want: LogLevelError},
		{name: "lowercase debug", envVal: "debug", want: LogLevelDebug},
		{name: "unknown value defaults to INFO", envVal: "NONSENSE", want: LogLevelInfo},
		{name: "unset defaults to INFO", unset: true, want: LogLevelInfo},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unset {
				os.Unsetenv("LOG_LEVEL")
			} else {
				t.Setenv("LOG_LEVEL", tc.envVal)
			}

			setLogLevelFromEnv()

			if currentLogLevel != tc.want {
				t.Errorf("setLogLevelFromEnv() with LOG_LEVEL=%q: currentLogLevel = %v, want %v", tc.envVal, currentLogLevel, tc.want)
			}
		})
	}
}

func TestGetLogLevelName(t *testing.T) {
	cases := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := getLogLevelName(tc.level); got != tc.want {
				t.Errorf("getLogLevelName(%v) = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
}

func TestSetLogLevel(t *testing.T) {
	restoreLoggerState(t)

	SetLogLevel(LogLevelError)

	if got := GetLogLevel(); got != LogLevelError {
		t.Errorf("GetLogLevel() = %v, want %v", got, LogLevelError)
	}
}

func TestIsDebugEnabled(t *testing.T) {
	restoreLoggerState(t)

	SetLogLevel(LogLevelDebug)
	if !IsDebugEnabled() {
		t.Error("expected IsDebugEnabled() = true when level is Debug")
	}

	SetLogLevel(LogLevelInfo)
	if IsDebugEnabled() {
		t.Error("expected IsDebugEnabled() = false when level is Info")
	}
}

// TestInitLogger_CreatesLogFileAndWritesToIt covers InitLogger's success path: a
// fresh working directory (so "logs/" is created from scratch), LOG_LEVEL unset
// (exercises the default-to-INFO branch), and confirms both the log file and the
// package-level Info logger actually receive output afterward.
func TestInitLogger_CreatesLogFileAndWritesToIt(t *testing.T) {
	restoreLoggerState(t)
	t.Chdir(t.TempDir())
	os.Unsetenv("LOG_LEVEL")

	InitLogger()

	entries, err := os.ReadDir("logs")
	if err != nil {
		t.Fatalf("expected InitLogger to create a logs/ directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 log file in logs/, got %d", len(entries))
	}

	logContent, err := os.ReadFile(filepath.Join("logs", entries[0].Name()))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(logContent), "Logger initialized with level: INFO") {
		t.Errorf("expected log file to contain the init message, got: %s", logContent)
	}

	var buf bytes.Buffer
	Info.SetOutput(&buf)
	Info.Printf("marker line")
	if !strings.Contains(buf.String(), "marker line") {
		t.Error("expected Info logger to still be usable after InitLogger")
	}
}
