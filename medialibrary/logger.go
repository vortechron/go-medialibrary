package medialibrary

import (
	"log"
	"os"
)

// LogLevel defines the level of logging
type LogLevel int

const (
	// LogLevelNone means no logging
	LogLevelNone LogLevel = iota
	// LogLevelError logs only errors
	LogLevelError
	// LogLevelWarning logs warnings and errors
	LogLevelWarning
	// LogLevelInfo logs info, warnings, and errors
	LogLevelInfo
	// LogLevelDebug logs everything
	LogLevelDebug
)

// Logger defines the interface for logging
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warning(format string, args ...interface{})
	Error(format string, args ...interface{})
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// DefaultLogger implements the Logger interface
type DefaultLogger struct {
	level  LogLevel
	logger *log.Logger
}

// NewDefaultLogger creates a new default logger with the specified log level
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		logger: log.New(os.Stdout, "MediaLibrary: ", log.LstdFlags),
	}
}

// Debug logs debug messages
func (l *DefaultLogger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs informational messages
func (l *DefaultLogger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Warning logs warning messages
func (l *DefaultLogger) Warning(format string, args ...interface{}) {
	if l.level >= LogLevelWarning {
		l.logger.Printf("[WARNING] "+format, args...)
	}
}

// Error logs error messages
func (l *DefaultLogger) Error(format string, args ...interface{}) {
	if l.level >= LogLevelError {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// SetLevel sets the logging level
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current logging level
func (l *DefaultLogger) GetLevel() LogLevel {
	return l.level
}
