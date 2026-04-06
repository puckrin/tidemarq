// Package watch manages per-job filesystem watchers.
// When files change under a watched source path, the registered callback is
// called after a short debounce period (the watcher waits until the directory
// has been quiet for debounceDuration before firing).
package watch

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDuration = 3 * time.Second

// TriggerFunc is called when a change is detected for a job.
type TriggerFunc func(jobID int64)

// Manager owns a single fsnotify.Watcher and routes events to per-job callbacks.
type Manager struct {
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	watches  map[string][]int64    // source path → job IDs
	triggers map[int64]TriggerFunc // job ID → callback
	timers   map[int64]*time.Timer // debounce timer per job
	done     chan struct{}
	closeOnce sync.Once
}

// New creates and starts a Manager.
func New() (*Manager, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	m := &Manager{
		watcher:  w,
		watches:  make(map[string][]int64),
		triggers: make(map[int64]TriggerFunc),
		timers:   make(map[int64]*time.Timer),
		done:     make(chan struct{}),
	}
	go m.loop()
	return m, nil
}

// Add registers a watch on sourcePath for jobID. fn is called after the debounce
// period whenever a change is detected. Watching the same path for multiple jobs
// is supported.
func (m *Manager) Add(jobID int64, sourcePath string, fn TriggerFunc) error {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	alreadyWatched := len(m.watches[abs]) > 0
	m.watches[abs] = append(m.watches[abs], jobID)
	m.triggers[jobID] = fn

	if !alreadyWatched {
		return m.watcher.Add(abs)
	}
	return nil
}

// Remove deregisters the watch for jobID.
func (m *Manager) Remove(jobID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.triggers, jobID)

	if t, ok := m.timers[jobID]; ok {
		t.Stop()
		delete(m.timers, jobID)
	}

	for path, ids := range m.watches {
		updated := ids[:0]
		for _, id := range ids {
			if id != jobID {
				updated = append(updated, id)
			}
		}
		if len(updated) == 0 {
			delete(m.watches, path)
			m.watcher.Remove(path) //nolint:errcheck
		} else {
			m.watches[path] = updated
		}
	}
}

// Close shuts down the manager and releases all resources. Safe to call multiple times.
func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		close(m.done)
		m.watcher.Close()
	})
}

func (m *Manager) loop() {
	for {
		select {
		case <-m.done:
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			m.handleEvent(event)
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watch: watcher error: %v", err)
		}
	}
}

func (m *Manager) handleEvent(event fsnotify.Event) {
	dir := filepath.Dir(event.Name)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find all jobs watching this directory (or a parent).
	for watchPath, ids := range m.watches {
		rel, err := filepath.Rel(watchPath, dir)
		if err != nil || (len(rel) > 1 && rel[:2] == "..") {
			continue
		}
		for _, id := range ids {
			m.debounce(id)
		}
	}
}

// debounce resets (or starts) the debounce timer for jobID. Must be called with m.mu held.
func (m *Manager) debounce(jobID int64) {
	if t, ok := m.timers[jobID]; ok {
		t.Reset(debounceDuration)
		return
	}
	fn := m.triggers[jobID]
	if fn == nil {
		return
	}
	m.timers[jobID] = time.AfterFunc(debounceDuration, func() {
		m.mu.Lock()
		delete(m.timers, jobID)
		m.mu.Unlock()
		fn(jobID)
	})
}
