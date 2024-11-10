package FileRotate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFileRotate(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "filerotate-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	opt := Options{
		MaxCount:     3,
		MaxSize:      100_000,
		ZStdCompress: true,
	}
	filename := filepath.Join(tmpDir, "test.log")
	fr, err := New(filename, opt)
	if err != nil {
		t.Fatalf("Cannot create rotate: %s", err)
	}
	defer fr.Close()
	// Make sure we write enough to rotate through MaxCount entries
	wg := sync.WaitGroup{}
	count := 50_000
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			fmt.Fprintf(fr, "Log entry %d\n", i)
			wg.Done()
		}()
	}
	wg.Wait()

	// Verify files were created and rotated
	files, err := filepath.Glob(filepath.Join(tmpDir, "test.log*"))
	if err != nil {
		t.Fatalf("Failed to list log files: %v", err)
	}
	t.Logf("Found %d log files", len(files))

	// Should have up to MaxCount files
	if len(files) > opt.MaxCount {
		t.Errorf("Too many log files: got %d, want <= %d", len(files), opt.MaxCount)
	}

	// When ZStdCompress=true, rotated files should be compressed
	compressedCount := 0
	foundBase := false
	for _, f := range files {
		if strings.HasSuffix(f, ".zst") {
			compressedCount++
		}
		if strings.HasSuffix(f, ".log") {
			foundBase = true
		}
	}
	if !foundBase {
		t.Errorf("Base log file not found")
	}
	if compressedCount != opt.MaxCount-1 {
		t.Errorf("Not enough compressed files found")
	}
	if len(files) != opt.MaxCount {
		t.Errorf("Expected %d files, got %d", opt.MaxCount, len(files))
	}
}
