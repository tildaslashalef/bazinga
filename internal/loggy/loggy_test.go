package loggy

import (
	"bytes"
	"context"
	"github.com/tildaslashalef/bazinga/internal/config"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestLoggingConfig(t *testing.T) {
	// Create a test config
	cfg := &config.LoggingConfig{
		Level:      "debug",
		Format:     "text",
		Output:     "console",
		AddSource:  true,
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     30,
	}

	// Reset loggy state for testing
	once = sync.Once{}
	logger = nil

	// Initialize with test config
	err := Init(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test that logger is created
	if logger == nil {
		t.Fatal("Logger should not be nil after initialization")
	}

	// Test log levels
	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug level should be enabled")
	}
}

func TestSourceAttribution(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Reset loggy state
	once = sync.Once{}
	logger = nil

	// Initialize with a test handler that writes to our buffer
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}
	handler := slog.NewTextHandler(&buf, opts)
	logger = slog.New(handler)

	// Call a function that logs
	testFunction()

	// Check that the log output contains source information
	output := buf.String()
	if !strings.Contains(output, "loggy_test.go") {
		t.Errorf("Log output should contain test file name 'loggy_test.go', got: %s", output)
	}
}

// testFunction is a helper for testing source attribution
func testFunction() {
	Debug("Test debug message", "key", "value")
}

func TestWithSourceLogger(t *testing.T) {
	// Reset loggy state
	once = sync.Once{}
	logger = nil

	// Initialize with default config
	err := InitDefault()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test WithSource function
	sourceLogger := WithSource()
	if sourceLogger == nil {
		t.Fatal("WithSource should not return nil")
	}

	if sourceLogger.Logger == nil {
		t.Fatal("WithSource Logger field should not be nil")
	}
}

func TestReconfigure(t *testing.T) {
	// Initialize with default config
	err := InitDefault()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	originalLogger := logger

	// Create new config
	newCfg := &config.LoggingConfig{
		Level:      "error",
		Format:     "json",
		Output:     "console",
		AddSource:  false,
		MaxSize:    20,
		MaxBackups: 10,
		MaxAge:     60,
	}

	// Reconfigure
	err = Reconfigure(newCfg)
	if err != nil {
		t.Fatalf("Failed to reconfigure logger: %v", err)
	}

	// Logger should be different now
	if logger == originalLogger {
		t.Error("Logger should be different after reconfiguration")
	}

	// Test that error level is enabled but debug is not
	if !logger.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error level should be enabled")
	}

	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug level should not be enabled when level is set to error")
	}
}
