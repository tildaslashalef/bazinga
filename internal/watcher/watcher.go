package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Operation Operation
	Timestamp time.Time
}

// Operation represents the type of file operation
type Operation string

const (
	OpCreate Operation = "create"
	OpModify Operation = "modify"
	OpDelete Operation = "delete"
	OpRename Operation = "rename"
)

// FileWatcher watches files for changes
type FileWatcher struct {
	watcher *fsnotify.Watcher
	events  chan FileEvent
	errors  chan error
	watched map[string]bool
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher: watcher,
		events:  make(chan FileEvent, 100),
		errors:  make(chan error, 10),
		watched: make(map[string]bool),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start watching in a goroutine
	go fw.watch()

	return fw, nil
}

// AddFile adds a file to be watched
func (fw *FileWatcher) AddFile(filePath string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if fw.watched[absPath] {
		return nil // Already watching
	}

	err = fw.watcher.Add(absPath)
	if err != nil {
		return fmt.Errorf("failed to add file to watcher: %w", err)
	}

	fw.watched[absPath] = true
	return nil
}

// RemoveFile removes a file from being watched
func (fw *FileWatcher) RemoveFile(filePath string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if !fw.watched[absPath] {
		return nil // Not watching
	}

	err = fw.watcher.Remove(absPath)
	if err != nil {
		return fmt.Errorf("failed to remove file from watcher: %w", err)
	}

	delete(fw.watched, absPath)
	return nil
}

// AddDirectory adds all files in a directory to be watched
func (fw *FileWatcher) AddDirectory(dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	return filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Skip files with errors
		}

		if !info.IsDir() {
			return fw.AddFile(path)
		}

		return nil
	})
}

// Events returns the channel for file events
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Errors returns the channel for errors
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// Close stops the file watcher
func (fw *FileWatcher) Close() error {
	fw.cancel()

	if fw.watcher != nil {
		return fw.watcher.Close()
	}

	return nil
}

// GetWatchedFiles returns a list of currently watched files
func (fw *FileWatcher) GetWatchedFiles() []string {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	var files []string
	for file := range fw.watched {
		files = append(files, file)
	}
	return files
}

// watch runs the main watching loop
func (fw *FileWatcher) watch() {
	defer close(fw.events)
	defer close(fw.errors)

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Filter out events we don't care about
			if fw.shouldIgnoreEvent(event) {
				continue
			}

			// Convert to our event type
			fileEvent := FileEvent{
				Path:      event.Name,
				Operation: fw.convertOperation(event.Op),
				Timestamp: time.Now(),
			}

			select {
			case fw.events <- fileEvent:
			case <-fw.ctx.Done():
				return
			default:
				// Drop event if channel is full
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

			select {
			case fw.errors <- err:
			case <-fw.ctx.Done():
				return
			default:
				// Drop error if channel is full
			}
		}
	}
}

// shouldIgnoreEvent determines if we should ignore certain file events
func (fw *FileWatcher) shouldIgnoreEvent(event fsnotify.Event) bool {
	// Ignore temporary files and backups
	name := filepath.Base(event.Name)

	// Common temporary file patterns
	if name[0] == '.' && name[len(name)-1] == '~' {
		return true
	}
	if name[0] == '#' && name[len(name)-1] == '#' {
		return true
	}
	if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
		return true
	}
	if name == ".DS_Store" || name == "Thumbs.db" {
		return true
	}

	// Ignore certain operations on directories
	if event.Op&fsnotify.Chmod == fsnotify.Chmod {
		return true // Ignore permission changes
	}

	return false
}

// convertOperation converts fsnotify operations to our operation type
func (fw *FileWatcher) convertOperation(op fsnotify.Op) Operation {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return OpCreate
	case op&fsnotify.Write == fsnotify.Write:
		return OpModify
	case op&fsnotify.Remove == fsnotify.Remove:
		return OpDelete
	case op&fsnotify.Rename == fsnotify.Rename:
		return OpRename
	default:
		return OpModify // Default to modify
	}
}
