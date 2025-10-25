package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/umputun/local-docs-mcp/app/server"
)

var revision = "unknown"

// Options defines command line options
type Options struct {
	SharedDocsDir  string        `long:"shared-docs-dir" env:"SHARED_DOCS_DIR" default:"~/.claude/commands" description:"shared documentation directory"`
	ProjectDocsDir string        `long:"docs-dir" env:"DOCS_DIR" default:"docs" description:"project docs directory"`
	EnableRootDocs bool          `long:"enable-root-docs" env:"ENABLE_ROOT_DOCS" description:"enable scanning root *.md files"`
	ExcludeDirs    []string      `long:"exclude-dir" env:"EXCLUDE_DIRS" env-delim:"," default:"plans" description:"directories to exclude from docs scan"`
	CacheTTL       time.Duration `long:"cache-ttl" env:"CACHE_TTL" default:"1h" description:"cache TTL (time-to-live) for file list"`
	MaxFileSize    int64         `long:"max-file-size" env:"MAX_FILE_SIZE" default:"5242880" description:"maximum file size in bytes to index"`
	Debug          bool          `long:"dbg" env:"DEBUG" description:"enable debug logging"`
}

func main() {
	var opts Options
	if _, err := flags.Parse(&opts); err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	// setup logging with text handler
	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	slog.Info("starting local-docs MCP server", "version", revision)

	// use embedded function to properly handle defer before os.Exit
	os.Exit(func() int {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer cancel()

		if err := run(ctx, opts); err != nil {
			slog.Error("fatal error", "error", err)
			return 1
		}
		return 0
	}())
}

func run(ctx context.Context, opts Options) error {
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
		MaxFileSize:    opts.MaxFileSize,
		ServerName:     "local-docs",
		Version:        revision,
		CacheTTL:       opts.CacheTTL,
	}

	// create server
	srv, err := server.New(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// run server
	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	slog.Info("server stopped")
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
