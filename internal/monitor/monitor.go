// Package monitor provides file system monitoring capabilities for watching
// Claude Code session directories and triggering callbacks on changes.
package monitor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Monitor watches a directory for file system changes and triggers callbacks.
type Monitor struct {
	watcher      *fsnotify.Watcher
	watchDir     string
	callback     func()
	stopCh       chan struct{}
	doneCh       chan struct{}
	mu           sync.RWMutex
	running      bool
	debounce     time.Duration
	logger       *log.Logger
	pendingMu    sync.Mutex
	pendingEvent bool
}

// Option configures the Monitor.
type Option func(*Monitor)

// WithDebounce sets the debounce duration for file events.
func WithDebounce(d time.Duration) Option {
	return func(m *Monitor) {
		m.debounce = d
	}
}

// WithLogger sets a custom logger for the monitor.
func WithLogger(logger *log.Logger) Option {
	return func(m *Monitor) {
		m.logger = logger
	}
}

// New creates a new Monitor for the specified directory.
// The callback is invoked when file changes are detected.
func New(watchDir string, callback func(), opts ...Option) (*Monitor, error) {
	if watchDir == "" {
		return nil, fmt.Errorf("monitor: watch directory cannot be empty")
	}

	if callback == nil {
		return nil, fmt.Errorf("monitor: callback cannot be nil")
	}

	// Verify directory exists
	info, err := os.Stat(watchDir)
	if err != nil {
		return nil, fmt.Errorf("monitor: cannot access watch directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("monitor: watch path is not a directory: %s", watchDir)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("monitor: failed to create watcher: %w", err)
	}

	m := &Monitor{
		watcher:  watcher,
		watchDir: watchDir,
		callback: callback,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		debounce: 500 * time.Millisecond,
		logger:   log.Default(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// Start begins watching the directory for changes.
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("monitor: already running")
	}
	m.running = true
	m.mu.Unlock()

	// Add the root directory and all subdirectories
	if err := m.addWatchRecursive(m.watchDir); err != nil {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return fmt.Errorf("monitor: failed to add watch: %w", err)
	}

	m.logger.Printf("Monitor started watching: %s", m.watchDir)

	go m.watchLoop(ctx)

	return nil
}

// Stop stops the monitor and cleans up resources.
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	close(m.stopCh)
	<-m.doneCh

	if err := m.watcher.Close(); err != nil {
		m.logger.Printf("Monitor: error closing watcher: %v", err)
	}

	m.logger.Printf("Monitor stopped")
}

// IsRunning returns whether the monitor is currently active.
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// watchLoop is the main event loop for the file watcher.
func (m *Monitor) watchLoop(ctx context.Context) {
	defer close(m.doneCh)

	var debounceTimer *time.Timer

	for {
		select {
		case <-m.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// 监听目录创建事件，自动添加到监控
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = m.addWatchRecursive(event.Name)
				}
			}

			// Only care about Create and Write events for .jsonl files
			if !m.isValidEvent(event) {
				continue
			}

			m.logger.Printf("Monitor: detected change in %s", event.Name)

			// Debounce rapid file changes
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			m.pendingMu.Lock()
			m.pendingEvent = true
			m.pendingMu.Unlock()

			debounceTimer = time.AfterFunc(m.debounce, func() {
				m.pendingMu.Lock()
				if m.pendingEvent {
					m.pendingEvent = false
					m.pendingMu.Unlock()
					m.triggerCallback()
				} else {
					m.pendingMu.Unlock()
				}
			})

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Printf("Monitor: watcher error: %v", err)
		}
	}
}

// isValidEvent checks if the file event should trigger a callback.
func (m *Monitor) isValidEvent(event fsnotify.Event) bool {
	// Only care about create and write operations
	if !(event.Op&fsnotify.Create != 0 || event.Op&fsnotify.Write != 0) {
		return false
	}

	// Only care about .jsonl files (Claude Code sessions)
	ext := filepath.Ext(event.Name)
	return ext == ".jsonl"
}

// triggerCallback invokes the registered callback function.
func (m *Monitor) triggerCallback() {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Printf("Monitor: callback panic recovered: %v", r)
		}
	}()

	m.callback()
}

// addWatchRecursive adds the directory and all subdirectories to the watcher.
func (m *Monitor) addWatchRecursive(dir string) error {
	// Add the directory itself
	if err := m.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch %s: %w", dir, err)
	}

	// Walk subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			if err := m.addWatchRecursive(subDir); err != nil {
				m.logger.Printf("Monitor: warning: %v", err)
				// Continue with other directories
			}
		}
	}

	return nil
}
