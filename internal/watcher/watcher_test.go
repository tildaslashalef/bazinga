package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestNewFileWatcher(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	if fw == nil {
		t.Fatal("FileWatcher should not be nil")
	}

	if fw.watcher == nil {
		t.Error("Internal watcher should not be nil")
	}

	if fw.events == nil {
		t.Error("Events channel should not be nil")
	}

	if fw.errors == nil {
		t.Error("Errors channel should not be nil")
	}

	if fw.watched == nil {
		t.Error("Watched map should not be nil")
	}
}

func TestFileWatcher_AddFile(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "watcher-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Test adding file
	err = fw.AddFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Check if file is in watched list
	watchedFiles := fw.GetWatchedFiles()
	found := false
	absPath, _ := filepath.Abs(tmpFile.Name())
	for _, file := range watchedFiles {
		if file == absPath {
			found = true
			break
		}
	}

	if !found {
		t.Error("File should be in watched list")
	}

	// Test adding same file again (should not error)
	err = fw.AddFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Adding same file twice should not error: %v", err)
	}
}

func TestFileWatcher_AddFile_NonExistent(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Test adding non-existent file
	err = fw.AddFile("/non/existent/file.txt")
	if err == nil {
		t.Error("Adding non-existent file should return error")
	}
}

func TestFileWatcher_RemoveFile(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "watcher-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Add file first
	err = fw.AddFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Remove file
	err = fw.RemoveFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Check if file is not in watched list
	watchedFiles := fw.GetWatchedFiles()
	absPath, _ := filepath.Abs(tmpFile.Name())
	for _, file := range watchedFiles {
		if file == absPath {
			t.Error("File should not be in watched list after removal")
		}
	}

	// Test removing file that's not watched (should not error)
	err = fw.RemoveFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Removing unwatched file should not error: %v", err)
	}
}

func TestFileWatcher_AddDirectory(t *testing.T) {
	// Create temporary directory with files
	tmpDir, err := os.MkdirTemp("", "watcher-test-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files in directory
	testFiles := []string{"file1.txt", "file2.go", "file3.md"}
	for _, fileName := range testFiles {
		filePath := filepath.Join(tmpDir, fileName)
		f, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
		_ = f.Close()
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Add directory
	err = fw.AddDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Failed to add directory: %v", err)
	}

	// Check if all files are watched
	watchedFiles := fw.GetWatchedFiles()
	if len(watchedFiles) != len(testFiles) {
		t.Errorf("Expected %d watched files, got %d", len(testFiles), len(watchedFiles))
	}

	// Verify each test file is watched
	for _, fileName := range testFiles {
		filePath := filepath.Join(tmpDir, fileName)
		absPath, _ := filepath.Abs(filePath)
		found := false
		for _, watchedFile := range watchedFiles {
			if watchedFile == absPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("File %s should be watched", fileName)
		}
	}
}

func TestFileWatcher_FileModification(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "watcher-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Add file to watcher
	err = fw.AddFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	_, err = tmpFile.WriteString("test content")
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	_ = tmpFile.Sync()

	// Wait for event
	select {
	case event := <-fw.Events():
		absPath, _ := filepath.Abs(tmpFile.Name())
		if event.Path != absPath {
			t.Errorf("Expected event for file %s, got %s", absPath, event.Path)
		}
		if event.Operation != OpModify {
			t.Errorf("Expected modify operation, got %s", event.Operation)
		}
		if event.Timestamp.IsZero() {
			t.Error("Event timestamp should not be zero")
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file modification event")
	}
}

func TestFileWatcher_GetWatchedFiles(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Initially should have no watched files
	watchedFiles := fw.GetWatchedFiles()
	if len(watchedFiles) != 0 {
		t.Errorf("Expected 0 watched files initially, got %d", len(watchedFiles))
	}

	// Create temporary files
	tmpFile1, err := os.CreateTemp("", "watcher-test1-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 1: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile1.Name()) }()
	defer func() { _ = tmpFile1.Close() }()

	tmpFile2, err := os.CreateTemp("", "watcher-test2-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 2: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile2.Name()) }()
	defer func() { _ = tmpFile2.Close() }()

	// Add files
	_ = fw.AddFile(tmpFile1.Name())
	_ = fw.AddFile(tmpFile2.Name())

	// Check watched files count
	watchedFiles = fw.GetWatchedFiles()
	if len(watchedFiles) != 2 {
		t.Errorf("Expected 2 watched files, got %d", len(watchedFiles))
	}
}

func TestFileWatcher_Close(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}

	// Test closing
	err = fw.Close()
	if err != nil {
		t.Errorf("Failed to close file watcher: %v", err)
	}

	// Test double close (should not panic)
	if err := fw.Close(); err != nil {
		// We don't assert error value here as it could be nil or non-nil on second close
		// depending on implementation
		t.Logf("Second close returned: %v", err)
	}
}

func TestFileWatcher_ShouldIgnoreEvent(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	tests := []struct {
		name     string
		fileName string
		op       fsnotify.Op
		expected bool
	}{
		{"temporary file with tilde", ".test~", fsnotify.Write, true},
		{"emacs backup file", "#test#", fsnotify.Write, true},
		{"temp file extension", "test.tmp", fsnotify.Write, true},
		{"temp file extension", "test.temp", fsnotify.Write, true},
		{"DS_Store", ".DS_Store", fsnotify.Write, true},
		{"Thumbs.db", "Thumbs.db", fsnotify.Write, true},
		{"chmod operation", "test.txt", fsnotify.Chmod, true},
		{"normal file", "test.txt", fsnotify.Write, false},
		{"normal create", "test.go", fsnotify.Create, false},
		{"normal remove", "test.md", fsnotify.Remove, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := fsnotify.Event{
				Name: tt.fileName,
				Op:   tt.op,
			}
			result := fw.shouldIgnoreEvent(event)
			if result != tt.expected {
				t.Errorf("shouldIgnoreEvent(%s, %v) = %v, expected %v", tt.fileName, tt.op, result, tt.expected)
			}
		})
	}
}

func TestFileWatcher_ConvertOperation(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	tests := []struct {
		name     string
		op       fsnotify.Op
		expected Operation
	}{
		{"create", fsnotify.Create, OpCreate},
		{"write", fsnotify.Write, OpModify},
		{"remove", fsnotify.Remove, OpDelete},
		{"rename", fsnotify.Rename, OpRename},
		{"chmod", fsnotify.Chmod, OpModify},                                            // Default to modify
		{"multiple ops", fsnotify.Create | fsnotify.Remove | fsnotify.Write, OpCreate}, // First match wins
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fw.convertOperation(tt.op)
			if result != tt.expected {
				t.Errorf("convertOperation(%v) = %v, expected %v", tt.op, result, tt.expected)
			}
		})
	}
}

func TestFileWatcher_Events_Channel(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	events := fw.Events()
	if events == nil {
		t.Error("Events channel should not be nil")
	}

	// Test that channel is receive-only
	select {
	case <-events:
		// This is expected behavior (might not receive anything immediately)
	default:
		// This is also fine
	}
}

func TestFileWatcher_Errors_Channel(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	errors := fw.Errors()
	if errors == nil {
		t.Error("Errors channel should not be nil")
	}

	// Test that channel is receive-only
	select {
	case <-errors:
		// This is expected behavior (might not receive anything immediately)
	default:
		// This is also fine
	}
}

func TestFileWatcher_ConcurrentAccess(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("Failed to create file watcher: %v", err)
	}
	defer func() { _ = fw.Close() }()

	// Create multiple temporary files
	var tmpFiles []*os.File
	for i := 0; i < 5; i++ {
		tmpFile, err := os.CreateTemp("", "watcher-concurrent-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file %d: %v", i, err)
		}
		tmpFiles = append(tmpFiles, tmpFile)
		defer func(f *os.File) {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}(tmpFile)
	}

	// Test concurrent add/remove operations
	done := make(chan bool, 10)

	// Add files concurrently
	for i := 0; i < 5; i++ {
		go func(file *os.File) {
			_ = fw.AddFile(file.Name())
			done <- true
		}(tmpFiles[i])
	}

	// Remove files concurrently
	for i := 0; i < 5; i++ {
		go func(file *os.File) {
			time.Sleep(10 * time.Millisecond) // Small delay
			_ = fw.RemoveFile(file.Name())
			done <- true
		}(tmpFiles[i])
	}

	// Wait for all operations to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}

	// Verify final state
	watchedFiles := fw.GetWatchedFiles()
	if len(watchedFiles) != 0 {
		t.Errorf("Expected 0 watched files after concurrent add/remove, got %d", len(watchedFiles))
	}
}
