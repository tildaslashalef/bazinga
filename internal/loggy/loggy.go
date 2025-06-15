package loggy

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger  *slog.Logger
	once    sync.Once
	logFile *lumberjack.Logger
)

// Logger provides a wrapper around slog.Logger with proper source attribution
type Logger struct {
	*slog.Logger
}

// Init initializes the loggy logger with configuration
func Init(cfg *config.LoggingConfig) error {
	var err error
	once.Do(func() {
		err = initLogger(cfg)
	})
	return err
}

// InitDefault initializes with default config if no config is provided
func InitDefault() error {
	defaultConfig := &config.LoggingConfig{
		Level:      "info",
		Format:     "text",
		Output:     "file",
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     30,
		AddSource:  true,
	}
	return Init(defaultConfig)
}

// NewTestLogger creates a logger suitable for tests that won't output anything
// Use this in test files to create a silent logger instance
func NewTestLogger() *Logger {
	// Create a handler that discards all output
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors and above
	})

	// Return a wrapped logger
	return &Logger{
		Logger: slog.New(handler),
	}
}

// initLogger sets up the logger based on configuration
func initLogger(cfg *config.LoggingConfig) error {
	// Parse log level
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Configure handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Fix source attribution to skip loggy wrapper functions
			if a.Key == slog.SourceKey && cfg.AddSource {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Skip frames that are in the loggy package
					if strings.Contains(source.File, "internal/loggy") {
						// Get the actual caller
						if actualSource := getActualSource(); actualSource != nil {
							return slog.Attr{
								Key:   slog.SourceKey,
								Value: slog.AnyValue(actualSource),
							}
						}
					}
				}
			}
			return a
		},
	}

	// Determine output destination
	var writer io.Writer
	switch strings.ToLower(cfg.Output) {
	case "console", "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	case "both":
		// Create combined writer for both file and console
		fileWriter, err := createFileWriter(cfg)
		if err != nil {
			return fmt.Errorf("failed to create file writer: %w", err)
		}
		writer = io.MultiWriter(fileWriter, os.Stdout)
	case "file":
		fallthrough
	default:
		fileWriter, err := createFileWriter(cfg)
		if err != nil {
			return fmt.Errorf("failed to create file writer: %w", err)
		}
		writer = fileWriter
	}

	// Create handler based on format
	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		fallthrough
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	logger = slog.New(handler)

	// Log initialization message
	logger.Info("Logger initialized",
		"level", cfg.Level,
		"format", cfg.Format,
		"output", cfg.Output,
		"add_source", cfg.AddSource,
	)

	return nil
}

// createFileWriter creates a file writer with rotation support
func createFileWriter(cfg *config.LoggingConfig) (io.Writer, error) {
	var logPath string

	if cfg.FilePath != "" {
		logPath = cfg.FilePath
	} else {
		// Get config directory
		configDir, err := config.GetConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get config directory: %w", err)
		}

		// Ensure config directory exists
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		logPath = filepath.Join(configDir, "bazinga.log")
	}

	// Create rotating file writer
	logFile = &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    cfg.MaxSize,    // megabytes
		MaxBackups: cfg.MaxBackups, // number of backups
		MaxAge:     cfg.MaxAge,     // days
		Compress:   true,           // compress rotated files
	}

	return logFile, nil
}

// getActualSource finds the actual source location, skipping loggy wrapper functions
func getActualSource() *slog.Source {
	// Start from frame 4 to skip runtime.Callers, getActualSource, ReplaceAttr, and the loggy wrapper
	pc := make([]uintptr, 10)
	n := runtime.Callers(4, pc)

	for i := 0; i < n; i++ {
		frame, _ := runtime.CallersFrames(pc[i : i+1]).Next()

		// Skip frames that are in the loggy package or slog internals
		if !strings.Contains(frame.File, "internal/loggy") &&
			!strings.Contains(frame.File, "log/slog") {
			return &slog.Source{
				Function: frame.Function,
				File:     frame.File,
				Line:     frame.Line,
			}
		}
	}

	return nil
}

// GetLogger returns the configured logger (initializes with defaults if needed)
func GetLogger() *slog.Logger {
	if logger == nil {
		if err := InitDefault(); err != nil {
			// Fallback to default logger if initialization fails
			logger = slog.Default()
			logger.Error("Failed to initialize loggy, using default logger", "error", err)
		}
	}
	return logger
}

// WithSource returns a logger that will correctly attribute the calling location
// This is the main function callers should use for proper source attribution
func WithSource() *Logger {
	return &Logger{Logger: GetLogger()}
}

// Debug logs a debug message with proper source attribution
func (l *Logger) Debug(msg string, args ...any) {
	if !l.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	// Get the actual caller information
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		l.Logger.Debug(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	// Create record with correct source information
	record := slog.NewRecord(time.Now(), slog.LevelDebug, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	// Add user attributes
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = l.Logger.Handler().Handle(context.Background(), record)
}

// Info logs an info message with proper source attribution
func (l *Logger) Info(msg string, args ...any) {
	if !l.Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		l.Logger.Info(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = l.Logger.Handler().Handle(context.Background(), record)
}

// Warn logs a warning message with proper source attribution
func (l *Logger) Warn(msg string, args ...any) {
	if !l.Enabled(context.Background(), slog.LevelWarn) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		l.Logger.Warn(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelWarn, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = l.Logger.Handler().Handle(context.Background(), record)
}

// Error logs an error message with proper source attribution
func (l *Logger) Error(msg string, args ...any) {
	if !l.Enabled(context.Background(), slog.LevelError) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		l.Logger.Error(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelError, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = l.Logger.Handler().Handle(context.Background(), record)
}

// Package-level convenience functions that maintain backward compatibility
// but still provide proper source attribution

// Debug logs a debug message
func Debug(msg string, args ...any) {
	if !GetLogger().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	// Get the actual caller (skip this function and go directly to caller)
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		GetLogger().Debug(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	// Create record with correct source information
	record := slog.NewRecord(time.Now(), slog.LevelDebug, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	// Add user attributes
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = GetLogger().Handler().Handle(context.Background(), record)
}

// Info logs an info message
func Info(msg string, args ...any) {
	if !GetLogger().Enabled(context.Background(), slog.LevelInfo) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		GetLogger().Info(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = GetLogger().Handler().Handle(context.Background(), record)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if !GetLogger().Enabled(context.Background(), slog.LevelWarn) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		GetLogger().Warn(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelWarn, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = GetLogger().Handler().Handle(context.Background(), record)
}

// Error logs an error message
func Error(msg string, args ...any) {
	if !GetLogger().Enabled(context.Background(), slog.LevelError) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		GetLogger().Error(msg, args...)
		return
	}

	fn := runtime.FuncForPC(pc)
	var funcName string
	if fn != nil {
		funcName = fn.Name()
	}

	record := slog.NewRecord(time.Now(), slog.LevelError, msg, pc)
	record.AddAttrs(slog.String("source_function", funcName))
	record.AddAttrs(slog.String("source_file", file))
	record.AddAttrs(slog.Int("source_line", line))

	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := args[i+1]
			record.AddAttrs(slog.Any(key, value))
		}
	}

	_ = GetLogger().Handler().Handle(context.Background(), record)
}

// With returns a logger with additional context
func With(args ...any) *slog.Logger {
	return GetLogger().With(args...)
}

// WithGroup returns a logger with a group name
func WithGroup(name string) *slog.Logger {
	return GetLogger().WithGroup(name)
}

// Close performs cleanup including closing log files
func Close() error {
	if logger != nil {
		Info("Closing loggy logger")
	}

	if logFile != nil {
		return logFile.Close()
	}

	return nil
}

// Reconfigure allows runtime reconfiguration of the logger
func Reconfigure(cfg *config.LoggingConfig) error {
	// Reset the once flag to allow reinitialization
	once = sync.Once{}
	logger = nil

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	return Init(cfg)
}
