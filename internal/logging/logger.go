package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel parses a string to a log level
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger represents a thread-safe logger
type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	file   *os.File
	json   bool
}

// New creates a new logger
func New(level Level, output io.Writer, file *os.File, jsonFormat bool) *Logger {
	return &Logger{
		level:  level,
		output: output,
		file:   file,
		json:   jsonFormat,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	l.log(LevelDebug, msg, fields)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.log(LevelInfo, msg, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	l.log(LevelWarn, msg, fields)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.log(LevelError, msg, fields)
}

// log logs a message at the specified level
func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var line string
	if l.json {
		entry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"level":     level.String(),
			"message":   msg,
		}
		for k, v := range fields {
			entry[k] = v
		}
		data, _ := json.Marshal(entry)
		line = string(data) + "\n"
	} else {
		line = fmt.Sprintf("[%s] [%s] %s", time.Now().UTC().Format(time.RFC3339), level.String(), msg)
		if len(fields) > 0 {
			data, _ := json.Marshal(fields)
			line += " " + string(data)
		}
		line += "\n"
	}

	// Write to console
	if l.output != nil {
		l.output.Write([]byte(line))
	}

	// Write to file if configured
	if l.file != nil {
		l.file.Write([]byte(line))
	}
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
