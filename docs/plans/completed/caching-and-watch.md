# Caching and File Watching Implementation Plan

## Overview

Add optional caching layer to scanner with automatic cache invalidation on file changes. This optimizes repeated queries while ensuring near real-time updates when documentation files change.

## Motivation

Current scanner reads filesystem on every query. With caching:
- Repeated searches/listings become 10x+ faster
- Network latency to MCP client reduced
- File system I/O minimized
- Still maintains fresh data through event-driven invalidation

## Architecture

### CachedScanner Wrapper

New `CachedScanner` type wraps existing `Scanner`:

```go
type CachedScanner struct {
    scanner     *Scanner
    cache       *cache.Cache  // github.com/go-pkgz/expirable-cache
    watcher     *fsnotify.Watcher
    debounceTimer *time.Timer
    stopCh      chan struct{}
}
```

Implements same `Scan(ctx) ([]FileInfo, error)` interface as Scanner.

### Cache Strategy

**Storage**: Single cache entry containing entire `[]FileInfo` slice
- Cache key: `"file_list"`
- TTL: 1 hour (fallback safety)
- Library: `github.com/go-pkgz/expirable-cache`

**Invalidation**: Event-driven via fsnotify
- Watch all three source directories
- On relevant file change: invalidate cache immediately
- Next Scan() call triggers rescan
- Debounce window: 500ms to handle multiple rapid events

### File Watching

**fsnotify Integration**:
- Monitor: commandsDir, projectDocsDir, projectRootDir
- Events: Create, Write, Remove, Rename
- Filter: Only `.md` files, exclude hidden dirs and `docs/plans/`
- Debouncing: 500ms timer resets on each event, invalidates on timer expiry

**Lifecycle**:
1. Startup: Create watcher, add directories, launch background goroutine
2. Running: Process events in goroutine, invalidate cache on changes
3. Shutdown: Stop watcher, close channels, clean up goroutine

### Scan Flow

```
Scan(ctx) called
    ↓
Check cache.Get(cacheKey)
    ↓
┌───────────────┐
│  Cache Hit?   │
└───────────────┘
    │         │
    Yes       No
    ↓         ↓
 Return   scanner.Scan(ctx)
 cached       ↓
 []FileInfo   cache.Set(cacheKey, files)
              ↓
           Return files
```

## Error Handling

**Watcher Initialization Failure**:
- Log error, continue without caching
- Degrade to direct scanner calls
- System remains functional

**Context Cancellation**:
- Watcher goroutine respects ctx.Done()
- Clean shutdown on cancellation
- Propagate context to underlying scanner

**TTL Fallback**:
- If events missed, cache expires after 1 hour
- Automatic rescan on next query
- Safety net for watcher failures

## Configuration

Add optional caching to server config:

```go
type Config struct {
    // ... existing fields
    EnableCache bool          `long:"enable-cache" env:"ENABLE_CACHE" description:"Enable file list caching"`
    CacheTTL    time.Duration `long:"cache-ttl" env:"CACHE_TTL" default:"1h" description:"Cache TTL"`
}
```

Server initialization:
```go
var scanner ScannerInterface
if config.EnableCache {
    scanner = NewCachedScanner(baseScanner, config.CacheTTL)
} else {
    scanner = baseScanner
}
```

## Testing Strategy

### Unit Tests
- Cache hit/miss behavior
- Invalidation logic
- Debouncing (multiple events → single invalidation)
- Context cancellation
- Graceful degradation

### Integration Tests
- Real filesystem operations in temp dir
- Create/modify/delete files, verify invalidation
- Verify rescan picks up changes
- Test with excluded directories
- Measure cache performance improvement

### Benchmarks
- Compare cached vs uncached Scan() times
- Target: 10x improvement for cache hits
- Memory usage profiling

## Implementation Steps

1. Add `github.com/fsnotify/fsnotify` dependency
2. Create `internal/scanner/cached.go` with CachedScanner
3. Implement Scan() with cache check
4. Implement startWatcher() with debouncing
5. Add Close() method for cleanup
6. Update scanner interface to include Close()
7. Add configuration options to server
8. Write unit tests for CachedScanner
9. Write integration tests with real files
10. Add benchmarks
11. Update documentation (README, ARCHITECTURE if exists)

## Performance Targets

- Cache hit latency: < 1ms (memory lookup)
- Cache miss latency: Same as current (10-50ms depending on file count)
- Invalidation delay: < 1 second from file change to cache clear
- Memory overhead: < 5MB for typical file list (100-1000 files)

## Dependencies

- `github.com/go-pkgz/expirable-cache` - already available per user
- `github.com/fsnotify/fsnotify` - standard filesystem notification library

## Future Enhancements

- Configurable debounce window
- Health check endpoint for watcher status
- Metrics: cache hit rate, invalidation frequency
- Incremental cache updates instead of full invalidation (complex, not needed initially)
