package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tildaslashalef/bazinga/internal/cli"
	"github.com/tildaslashalef/bazinga/internal/loggy"
)

var (
	Version    = "dev"
	CommitHash = "unknown"
	BuildTime  = "unknown"
	Author     = "unknown"
	Email      = "unknown"
)

func main() {
	// Initialize logging first with default config
	// We'll reconfigure it later with the loaded config
	if err := loggy.InitDefault(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
	}
	defer func() {
		if err := loggy.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to close logger: %v\n", err)
		}
	}()

	loggy.Info("Starting bazinga",
		"version", Version,
		"commit", CommitHash,
		"date", BuildTime,
		"author", Author,
		"email", Email,
	)

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		loggy.Info("Received shutdown signal")
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		cancel()
	}()

	// Create and execute CLI
	rootCmd := cli.NewRootCommand(&cli.BuildInfo{
		Version: Version,
		Commit:  CommitHash,
		Date:    BuildTime,
		Author:  Author,
		Email:   Email,
	})

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		loggy.Error("CLI execution failed", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	loggy.Info("Bazinga shutdown complete")
}
