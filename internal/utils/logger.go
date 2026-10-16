package utils

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thd-spatial-ai/city2tabula/internal/config"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger is the structured logger: callers that know which run, country or
// dataset a line belongs to add those as fields (Logger.With or key-value
// pairs). Info, Debug, Warn and Error are printf-style views of the same
// handler, so every line has one format, level and time whichever API wrote it.
//
// They start with safe defaults so code that uses them never panics, even in
// tests that don't call InitLogger(). InitLogger() adds the log file and reads
// LOG_LEVEL and LOG_FORMAT.
var (
	level  = new(slog.LevelVar)
	Logger = slog.New(newHandler(os.Stdout, "text"))

	Info  = slog.NewLogLogger(Logger.Handler(), slog.LevelInfo)
	Debug = slog.NewLogLogger(Logger.Handler(), slog.LevelDebug)
	Warn  = slog.NewLogLogger(Logger.Handler(), slog.LevelWarn)
	Error = slog.NewLogLogger(Logger.Handler(), slog.LevelError)

	// Current log level - can be configured
	currentLogLevel LogLevel = LogLevelInfo // Default to INFO level
)

// newHandler returns a JSON handler for format "json" and a text (logfmt)
// handler otherwise. Both stamp the level, time and caller's file:line, and
// drop records below the current level.
func newHandler(w io.Writer, format string) slog.Handler {
	opts := &slog.HandlerOptions{AddSource: true, Level: level, ReplaceAttr: shortSource}
	if format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// shortSource trims the caller's absolute path to package/file.go:line, which
// identifies the line without the build machine's directory layout.
func shortSource(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.SourceKey {
		return a
	}
	src, ok := a.Value.Any().(*slog.Source)
	if !ok || src == nil {
		return a
	}
	file := filepath.Join(filepath.Base(filepath.Dir(src.File)), filepath.Base(src.File))
	return slog.String(slog.SourceKey, file+":"+strconv.Itoa(src.Line))
}

// setLogger makes l the logger behind Logger, the printf-style loggers and the
// standard library's default logger.
func setLogger(l *slog.Logger) {
	Logger = l
	Info = slog.NewLogLogger(l.Handler(), slog.LevelInfo)
	Debug = slog.NewLogLogger(l.Handler(), slog.LevelDebug)
	Warn = slog.NewLogLogger(l.Handler(), slog.LevelWarn)
	Error = slog.NewLogLogger(l.Handler(), slog.LevelError)
	slog.SetDefault(l)
}

// InitLogger sets the level from LOG_LEVEL and the format from LOG_FORMAT
// (text or json), and writes every line to stdout and to logs/<date>.log.
func InitLogger() {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	setLogLevelFromEnv()
	format := logFormatFromEnv()

	logFileName := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	setLogger(slog.New(newHandler(io.MultiWriter(os.Stdout, logFile), format)))
	Info.Printf("Logger initialized with level: %s, format: %s", getLogLevelName(currentLogLevel), format)
}

// logFormatFromEnv returns LOG_FORMAT lower-cased, or "text" when it is unset
// or not one of text and json.
func logFormatFromEnv() string {
	format := strings.ToLower(config.GetEnv("LOG_FORMAT", "text"))
	if format != "text" && format != "json" {
		log.Printf("Unknown log format '%s', defaulting to text", format)
		return "text"
	}
	return format
}

// setLogLevelFromEnv sets the log level from LOG_LEVEL environment variable
func setLogLevelFromEnv() {
	logLevelStr := strings.ToUpper(config.GetEnv("LOG_LEVEL", "INFO"))

	switch logLevelStr {
	case "DEBUG":
		setLevel(LogLevelDebug)
	case "INFO":
		setLevel(LogLevelInfo)
	case "WARN", "WARNING":
		setLevel(LogLevelWarn)
	case "ERROR":
		setLevel(LogLevelError)
	default:
		setLevel(LogLevelInfo)
		log.Printf("Unknown log level '%s', defaulting to INFO", logLevelStr)
	}
}

// setLevel records l and applies it to the handler.
func setLevel(l LogLevel) {
	currentLogLevel = l
	switch l {
	case LogLevelDebug:
		level.Set(slog.LevelDebug)
	case LogLevelWarn:
		level.Set(slog.LevelWarn)
	case LogLevelError:
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
}

// getLogLevelName returns the name of the log level
func getLogLevelName(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// SetLogLevel allows programmatic control of log level
func SetLogLevel(level LogLevel) {
	setLevel(level)
	Info.Printf("Log level changed to: %s", getLogLevelName(level))
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return currentLogLevel <= LogLevelDebug
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	return currentLogLevel
}
