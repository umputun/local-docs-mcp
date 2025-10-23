package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/umputun/local-docs-mcp/internal/server"
)

var revision = "unknown"

const (
	maxFileSize = 5 * 1024 * 1024 // 5MB
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

func run() error {
	// setup logging
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[local-docs-mcp] ")

	// determine directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	commandsDir := filepath.Join(homeDir, ".claude", "commands")
	projectDocsDir := filepath.Join(cwd, "docs")
	projectRootDir := cwd

	// create server config
	config := server.Config{
		CommandsDir:    commandsDir,
		ProjectDocsDir: projectDocsDir,
		ProjectRootDir: projectRootDir,
		MaxFileSize:    maxFileSize,
		ServerName:     "local-docs",
		Version:        revision,
	}

	// create server
	srv, err := server.New(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("[INFO] shutdown signal received")
		cancel()
	}()

	// run server
	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	log.Printf("[INFO] server stopped")
	return nil
}
