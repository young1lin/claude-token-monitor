package monitor

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatcherInterface defines the interface for file watchers
type WatcherInterface interface {
	Lines() <-chan string
	Errors() <-chan error
	Close() error
}

// Watcher monitors a session file for changes
type Watcher struct {
	watcher   *fsnotify.Watcher
	filePath  string
	offset    int64
	linesChan chan string
	errorChan chan error
	done      chan struct{}
}

// NewWatcher creates a new file watcher for a session file
func NewWatcher(filePath string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Get initial file offset
	offset, err := GetFileOffset(filePath)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}

	// Watch the directory containing the file
	dirPath := "." // Could extract dir from filePath
	if err := fsWatcher.Add(dirPath); err != nil {
		fsWatcher.Close()
		return nil, err
	}

	w := &Watcher{
		watcher:   fsWatcher,
		filePath:  filePath,
		offset:    offset,
		linesChan: make(chan string, 100),
		errorChan: make(chan error, 10),
		done:      make(chan struct{}),
	}

	go w.watch()

	return w, nil
}

// watch runs the file watching loop
func (w *Watcher) watch() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer close(w.linesChan)
	defer close(w.errorChan)

	for {
		select {
		case <-w.done:
			return

		case <-ticker.C:
			// Periodically check for new content (polling as backup)
			w.checkForNewContent()

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our file
			if event.Name == w.filePath {
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					w.checkForNewContent()
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.errorChan <- err
		}
	}
}

// checkForNewContent reads and sends any new lines from the file
func (w *Watcher) checkForNewContent() {
	lines, newOffset, err := TailFile(w.filePath, w.offset)
	if err != nil {
		w.errorChan <- err
		return
	}

	for _, line := range lines {
		w.linesChan <- line
	}

	w.offset = newOffset
}

// Lines returns a channel of new lines as they are written to the file
func (w *Watcher) Lines() <-chan string {
	return w.linesChan
}

// Errors returns a channel of errors that occur during watching
func (w *Watcher) Errors() <-chan error {
	return w.errorChan
}

// Close stops watching the file
func (w *Watcher) Close() error {
	select {
	case <-w.done:
		// Already closed
		return nil
	default:
		close(w.done)
	}
	return w.watcher.Close()
}

// TestWatcher is a helper for testing that provides direct control over channels
type TestWatcher struct {
	linesChan    chan string
	errorChan    chan error
	closeChan    chan struct{}
	closed       bool
	linesClosed  bool
	errorsClosed bool
	mu           sync.Mutex
}

// NewTestWatcher creates a test watcher with controllable channels
func NewTestWatcher() *TestWatcher {
	return &TestWatcher{
		linesChan: make(chan string, 100),
		errorChan: make(chan error, 10),
		closeChan: make(chan struct{}),
	}
}

func (tw *TestWatcher) Lines() <-chan string {
	return tw.linesChan
}

func (tw *TestWatcher) Errors() <-chan error {
	return tw.errorChan
}

func (tw *TestWatcher) Close() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.closed {
		return nil
	}
	tw.closed = true

	// Only close channels that haven't been closed yet
	if !tw.linesClosed {
		close(tw.linesChan)
		tw.linesClosed = true
	}
	if !tw.errorsClosed {
		close(tw.errorChan)
		tw.errorsClosed = true
	}
	close(tw.closeChan)
	return nil
}

// CloseLinesOnly closes only the Lines channel (used for testing specific branches)
func (tw *TestWatcher) CloseLinesOnly() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.linesClosed {
		close(tw.linesChan)
		tw.linesClosed = true
	}
}

// CloseErrorsOnly closes only the Errors channel (used for testing specific branches)
func (tw *TestWatcher) CloseErrorsOnly() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.errorsClosed {
		close(tw.errorChan)
		tw.errorsClosed = true
	}
}

// SendLine sends a test line to the watcher
func (tw *TestWatcher) SendLine(line string) {
	tw.linesChan <- line
}

// SendError sends a test error to the watcher
func (tw *TestWatcher) SendError(err error) {
	tw.errorChan <- err
}
