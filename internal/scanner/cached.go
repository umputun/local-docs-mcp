package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	cache "github.com/go-pkgz/expirable-cache/v3"
)

const (
	cacheKey         = "file_list"
	defaultCacheTTL  = 1 * time.Hour
	defaultDebounce  = 500 * time.Millisecond
	watchBufferSize  = 100
	invalidateBuffer = 10
)

// CachedScanner wraps Scanner with caching and file watching capabilities
type CachedScanner struct {
	scanner       *Scanner
	cache         cache.Cache[string, []FileInfo]
	watcher       *fsnotify.Watcher
	stopCh        chan struct{}
	invalidateCh  chan struct{}
	mu            sync.RWMutex
	ttl           time.Duration
	debounce      time.Duration
	watcherActive bool
}

// NewCachedScanner creates a new cached scanner with file watching
func NewCachedScanner(scanner *Scanner, ttl time.Duration) (*CachedScanner, error) {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	cs := &CachedScanner{
		scanner:      scanner,
		cache:        cache.NewCache[string, []FileInfo]().WithTTL(ttl),
		stopCh:       make(chan struct{}),
		invalidateCh: make(chan struct{}, invalidateBuffer),
		ttl:          ttl,
		debounce:     defaultDebounce,
	}

	// attempt to start watcher, but don't fail if it doesn't work
	if err := cs.startWatcher(context.Background()); err != nil {
		// log error but continue without watching
		// scanner will work, just without automatic cache invalidation
		return cs, nil
	}

	return cs, nil
}

// Scan returns cached file list or scans filesystem if cache miss
func (cs *CachedScanner) Scan(ctx context.Context) ([]FileInfo, error) {
	// check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err() // nolint:wrapcheck // context errors should be returned as-is
	default:
	}

	// try cache first
	if files, ok := cs.cache.Get(cacheKey); ok {
		return files, nil
	}

	// cache miss - scan filesystem
	files, err := cs.scanner.Scan(ctx)
	if err != nil {
		return nil, err
	}

	// populate cache
	cs.cache.Set(cacheKey, files, cs.ttl)
	return files, nil
}

// CommandsDir returns the commands directory path
func (cs *CachedScanner) CommandsDir() string {
	return cs.scanner.CommandsDir()
}

// ProjectDocsDir returns the project docs directory path
func (cs *CachedScanner) ProjectDocsDir() string {
	return cs.scanner.ProjectDocsDir()
}

// ProjectRootDir returns the project root directory path
func (cs *CachedScanner) ProjectRootDir() string {
	return cs.scanner.ProjectRootDir()
}

// Close stops the file watcher and cleans up resources
func (cs *CachedScanner) Close() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.watcherActive {
		return nil
	}

	close(cs.stopCh)
	cs.watcherActive = false

	if cs.watcher != nil {
		return cs.watcher.Close() // nolint:wrapcheck // watcher error is descriptive
	}

	return nil
}

// startWatcher initializes fsnotify watcher and monitoring goroutine
func (cs *CachedScanner) startWatcher(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	cs.watcher = watcher

	// add all source directories to watcher
	dirs := []string{
		cs.scanner.CommandsDir(),
		cs.scanner.ProjectDocsDir(),
		cs.scanner.ProjectRootDir(),
	}

	for _, dir := range dirs {
		if err := cs.addWatchRecursive(dir); err != nil {
			// log but don't fail - some dirs might not exist
			continue
		}
	}

	cs.mu.Lock()
	cs.watcherActive = true
	cs.mu.Unlock()

	// start monitoring goroutine
	go cs.watchLoop(ctx)

	return nil
}

// addWatchRecursive adds directory and subdirectories to watcher
func (cs *CachedScanner) addWatchRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error { // nolint:wrapcheck // filepath.Walk error is descriptive
		if err != nil {
			return nil // skip errors
		}

		// skip hidden directories
		if filepath.Base(path) != "" && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// skip excluded directories
		if info.IsDir() && cs.scanner.shouldExcludeDir(filepath.Base(path)) {
			return filepath.SkipDir
		}

		// add directory to watcher (watch directories only)
		if info.IsDir() {
			if err := cs.watcher.Add(path); err != nil {
				return nil // skip if can't add
			}
		}

		return nil
	})
}

// watchLoop processes file system events with debouncing
func (cs *CachedScanner) watchLoop(ctx context.Context) {
	debounceTimer := time.NewTimer(cs.debounce)
	debounceTimer.Stop() // stop initial timer

	for {
		select {
		case <-ctx.Done():
			return

		case <-cs.stopCh:
			return

		case event, ok := <-cs.watcher.Events:
			if !ok {
				return
			}

			if cs.isRelevantEvent(event) {
				// reset debounce timer on each relevant event
				debounceTimer.Reset(cs.debounce)
			}

		case <-debounceTimer.C:
			// debounce period elapsed, invalidate cache
			cs.invalidate()

		case err, ok := <-cs.watcher.Errors:
			if !ok {
				return
			}
			// log error but continue watching
			_ = err
		}
	}
}

// isRelevantEvent checks if event should trigger cache invalidation
func (cs *CachedScanner) isRelevantEvent(event fsnotify.Event) bool {
	// only care about write, create, remove, rename
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) &&
		!event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
		return false
	}

	// only care about .md files
	if !strings.HasSuffix(event.Name, ".md") {
		return false
	}

	// skip hidden files
	base := filepath.Base(event.Name)
	if strings.HasPrefix(base, ".") {
		return false
	}

	// skip excluded directories
	if cs.isInExcludedPath(event.Name) {
		return false
	}

	return true
}

// invalidate clears the cache
func (cs *CachedScanner) invalidate() {
	cs.cache.Invalidate(cacheKey)
}

// isInExcludedPath checks if file path contains any excluded directory
func (cs *CachedScanner) isInExcludedPath(path string) bool {
	// normalize path to use forward slashes for cross-platform compatibility
	normalizedPath := filepath.ToSlash(path)
	pathParts := strings.Split(normalizedPath, "/")

	// check if any path component matches excluded directories
	for _, part := range pathParts {
		if cs.scanner.shouldExcludeDir(part) {
			return true
		}
	}
	return false
}
