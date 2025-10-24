package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCachedScanner(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	require.NotNil(t, cached)

	defer cached.Close()

	assert.NotNil(t, cached.cache)
	assert.NotNil(t, cached.scanner)
	assert.Equal(t, 1*time.Hour, cached.ttl)
}

func TestCachedScanner_Scan_CacheHitMiss(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create test file
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	defer cached.Close()

	ctx := context.Background()

	// first scan - cache miss
	start := time.Now()
	files1, err := cached.Scan(ctx)
	duration1 := time.Since(start)
	require.NoError(t, err)
	require.Len(t, files1, 1)

	// second scan - cache hit (should be much faster)
	start = time.Now()
	files2, err := cached.Scan(ctx)
	duration2 := time.Since(start)
	require.NoError(t, err)
	require.Len(t, files2, 1)

	// cache hit should be significantly faster
	assert.Less(t, duration2, duration1/2, "cached scan should be faster")
	assert.Equal(t, files1[0].Name, files2[0].Name)
}

func TestCachedScanner_Invalidate(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	defer cached.Close()

	ctx := context.Background()

	// populate cache
	files1, err := cached.Scan(ctx)
	require.NoError(t, err)
	require.Len(t, files1, 1)

	// verify cache hit
	files2, err := cached.Scan(ctx)
	require.NoError(t, err)
	require.Len(t, files2, 1)

	// invalidate cache
	cached.invalidate()

	// next scan should be cache miss and rescan
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "new.md"), []byte("new"), 0600))
	files3, err := cached.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, files3, 2, "should see new file after invalidation")
}

func TestCachedScanner_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	defer cached.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = cached.Scan(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestCachedScanner_Close(t *testing.T) {
	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)

	// close should not error
	err = cached.Close()
	assert.NoError(t, err)

	// second close should not error
	err = cached.Close()
	assert.NoError(t, err)
}

func TestCachedScanner_IsRelevantEvent(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewScanner(Params{CommandsDir: tmpDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	defer cached.Close()

	tests := []struct {
		name     string
		path     string
		op       string
		expected bool
	}{
		{
			name:     "markdown file write",
			path:     "/path/to/test.md",
			op:       "write",
			expected: true,
		},
		{
			name:     "markdown file create",
			path:     "/path/to/test.md",
			op:       "create",
			expected: true,
		},
		{
			name:     "non-markdown file",
			path:     "/path/to/test.txt",
			op:       "write",
			expected: false,
		},
		{
			name:     "hidden file",
			path:     "/path/to/.hidden.md",
			op:       "write",
			expected: false,
		},
		{
			name:     "plans directory",
			path:     "/path/docs/plans/plan.md",
			op:       "write",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create actual fsnotify.Event
			var op fsnotify.Op
			switch tt.op {
			case "write":
				op = fsnotify.Write
			case "create":
				op = fsnotify.Create
			case "remove":
				op = fsnotify.Remove
			case "rename":
				op = fsnotify.Rename
			default:
				op = fsnotify.Write
			}

			event := fsnotify.Event{
				Name: tt.path,
				Op:   op,
			}

			result := cached.isRelevantEvent(event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCachedScanner_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	// create initial file
	testFile := filepath.Join(commandsDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("initial"), 0600))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(t, err)
	defer cached.Close()

	ctx := context.Background()

	// initial scan
	files, err := cached.Scan(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)

	// modify file - should trigger invalidation
	require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0600))

	// give watcher time to process event and debounce
	time.Sleep(1 * time.Second)

	// scan should see modification (new rescan)
	files, err = cached.Scan(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestCachedScanner_TTLExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL test in short mode")
	}

	tmpDir := t.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "test.md"), []byte("test"), 0600))

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	// use very short TTL for testing
	cached, err := NewCachedScanner(scanner, 100*time.Millisecond)
	require.NoError(t, err)
	defer cached.Close()

	ctx := context.Background()

	// populate cache
	files1, err := cached.Scan(ctx)
	require.NoError(t, err)
	require.Len(t, files1, 1)

	// wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// add new file
	require.NoError(t, os.WriteFile(filepath.Join(commandsDir, "new.md"), []byte("new"), 0600))

	// scan should see new file (cache expired)
	files2, err := cached.Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, files2, 2, "should see new file after TTL expiration")
}

func BenchmarkScanner_Scan(b *testing.B) {
	tmpDir := b.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(b, os.MkdirAll(commandsDir, 0755))

	// create test files
	for i := 0; i < 50; i++ {
		filename := filepath.Join(commandsDir, fmt.Sprintf("test-%d.md", i))
		require.NoError(b, os.WriteFile(filename, []byte("test content"), 0600))
	}

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedScanner_ScanCacheMiss(b *testing.B) {
	tmpDir := b.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(b, os.MkdirAll(commandsDir, 0755))

	// create test files
	for i := 0; i < 50; i++ {
		filename := filepath.Join(commandsDir, fmt.Sprintf("test-%d.md", i))
		require.NoError(b, os.WriteFile(filename, []byte("test content"), 0600))
	}

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	ctx := context.Background()

	b.ResetTimer()
	// run limited iterations to avoid file descriptor exhaustion
	// benchmark cache miss by creating fresh scanner each time
	iterations := b.N
	if iterations > 100 {
		iterations = 100
	}
	for i := 0; i < iterations; i++ {
		b.StopTimer()
		cached, err := NewCachedScanner(scanner, 1*time.Hour)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		_, err = cached.Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		cached.Close()
		b.StartTimer()
	}
}

func BenchmarkCachedScanner_ScanCacheHit(b *testing.B) {
	tmpDir := b.TempDir()
	commandsDir := filepath.Join(tmpDir, "commands")
	require.NoError(b, os.MkdirAll(commandsDir, 0755))

	// create test files
	for i := 0; i < 50; i++ {
		filename := filepath.Join(commandsDir, fmt.Sprintf("test-%d.md", i))
		require.NoError(b, os.WriteFile(filename, []byte("test content"), 0600))
	}

	scanner := NewScanner(Params{CommandsDir: commandsDir, MaxFileSize: 1024 * 1024, ExcludeDirs: []string{"plans"}})
	cached, err := NewCachedScanner(scanner, 1*time.Hour)
	require.NoError(b, err)
	defer cached.Close()

	ctx := context.Background()

	// warm up cache
	_, err = cached.Scan(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cached.Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
