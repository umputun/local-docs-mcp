package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/umputun/local-docs-mcp/internal/server"
)

var revision = "unknown"

const (
	maxFileSize = 5 * 1024 * 1024 // 5MB
)

// Options defines command line options
type Options struct {
	SharedDocsDir  string        `long:"shared-docs-dir" env:"SHARED_DOCS_DIR" default:"~/.claude/commands" description:"shared documentation directory"`
	ProjectDocsDir string        `long:"docs-dir" env:"DOCS_DIR" default:"docs" description:"project docs directory"`
	EnableRootDocs bool          `long:"enable-root-docs" env:"ENABLE_ROOT_DOCS" description:"enable scanning root *.md files"`
	ExcludeDirs    []string      `long:"exclude-dir" env:"EXCLUDE_DIRS" env-delim:"," default:"plans" description:"directories to exclude from docs scan"`
	EnableCache    bool          `long:"enable-cache" env:"ENABLE_CACHE" description:"enable file list caching with automatic invalidation"`
	CacheTTL       time.Duration `long:"cache-ttl" env:"CACHE_TTL" default:"1h" description:"cache TTL (time-to-live) for file list"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

func run() error {
	// setup logging
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[local-docs-mcp] ")

	// parse command line options
	var opts Options
	if _, err := flags.Parse(&opts); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return nil
		}
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// expand ~ in shared docs dir
	sharedDocsDir, err := expandTilde(opts.SharedDocsDir)
	if err != nil {
		return err
	}

	// get current directory (project root)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// project docs dir is relative to cwd
	projectDocsDir := filepath.Join(cwd, opts.ProjectDocsDir)

	// project root dir - only used if EnableRootDocs is true
	projectRootDir := ""
	if opts.EnableRootDocs {
		projectRootDir = cwd
	}

	// create server config
	config := server.Config{
		CommandsDir:    sharedDocsDir,
		ProjectDocsDir: projectDocsDir,
		ProjectRootDir: projectRootDir,
		ExcludeDirs:    opts.ExcludeDirs,
		MaxFileSize:    maxFileSize,
		ServerName:     "local-docs",
		Version:        revision,
		EnableCache:    opts.EnableCache,
		CacheTTL:       opts.CacheTTL,
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

// expandTilde expands ~ prefix in path to user home directory
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, path[2:]), nil
}
