package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Level controls which messages a Logger emits.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

func (level Level) String() string {
	switch level {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	default:
		return "???"
	}
}

// Logger is a simple leveled logger that writes to stderr.
type Logger struct {
	level  Level
	output io.Writer
}

// NewLogger creates a Logger at the given level.
func NewLogger(level Level) *Logger {
	return &Logger{level: level, output: os.Stderr}
}

func (logger *Logger) log(level Level, format string, args ...any) {
	if level > logger.level {
		return
	}
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Fprintf(logger.output, "%s [%s] %s\n", timestamp, level, message)
}

// Error logs at ERROR level.
func (logger *Logger) Error(format string, args ...any) {
	logger.log(LevelError, format, args...)
}

// Warn logs at WARN level.
func (logger *Logger) Warn(format string, args ...any) {
	logger.log(LevelWarn, format, args...)
}

// Info logs at INFO level.
func (logger *Logger) Info(format string, args ...any) {
	logger.log(LevelInfo, format, args...)
}

// Debug logs at DEBUG level.
func (logger *Logger) Debug(format string, args ...any) {
	logger.log(LevelDebug, format, args...)
}
