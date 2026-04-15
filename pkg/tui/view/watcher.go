package view

import (
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// rulesFileChangedMsg is sent when the watched rules file changes on disk.
type rulesFileChangedMsg struct{}

// RulesWatcher watches the parent directory of the active rules file for
// changes and emits rulesFileChangedMsg via a channel that integrates with
// Bubble Tea's command system. It watches the parent directory (not the file
// itself) because editors perform atomic saves via temp-file + rename, which
// destroys direct file watches.
type RulesWatcher struct {
	watcher    *fsnotify.Watcher
	targetPath string // absolute path to the rules file being watched
	watchedDir string // directory currently added to fsnotify
	ch         chan tea.Msg
	mu         sync.Mutex
	closed     bool

	// debounce state — guarded by mu
	debounceTimer *time.Timer
}

// NewRulesWatcher creates a new RulesWatcher. The watcher does not begin
// watching any file until SetTarget is called.
func NewRulesWatcher() (*RulesWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	rw := &RulesWatcher{
		watcher: w,
		ch:      make(chan tea.Msg, 1),
	}
	go rw.loop()
	return rw, nil
}

// SetTarget updates the file being watched. If the parent directory changes,
// the old directory watch is removed and a new one is added.
func (rw *RulesWatcher) SetTarget(absPath string) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.closed || absPath == "" {
		return
	}

	dir := filepath.Dir(absPath)
	if dir == rw.watchedDir && absPath == rw.targetPath {
		return // already watching
	}

	// Update target first so the loop filters for the new filename.
	rw.targetPath = absPath

	if dir != rw.watchedDir {
		if rw.watchedDir != "" {
			_ = rw.watcher.Remove(rw.watchedDir)
		}
		if err := rw.watcher.Add(dir); err == nil {
			rw.watchedDir = dir
		}
	}
}

// NextEvent returns a Bubble Tea command that blocks until the next
// rulesFileChangedMsg is available. Re-queue this after each receipt to
// keep listening.
func (rw *RulesWatcher) NextEvent() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-rw.ch
		if !ok {
			return nil
		}
		return msg
	}
}

// Close shuts down the watcher and its background goroutine.
func (rw *RulesWatcher) Close() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.closed {
		return
	}
	rw.closed = true
	if rw.debounceTimer != nil {
		rw.debounceTimer.Stop()
	}
	rw.watcher.Close()
	close(rw.ch)
}

// loop is the background goroutine that reads fsnotify events, filters for
// the target filename, debounces rapid writes, and pushes messages to ch.
func (rw *RulesWatcher) loop() {
	for {
		select {
		case event, ok := <-rw.watcher.Events:
			if !ok {
				return
			}

			// Skip CHMOD-only events (macOS fires these frequently).
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}

			rw.mu.Lock()
			target := rw.targetPath
			rw.mu.Unlock()

			if target == "" {
				continue
			}

			// Filter: only care about events matching the target filename.
			if filepath.Base(event.Name) != filepath.Base(target) {
				continue
			}

			rw.debounce()

		case _, ok := <-rw.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// debounce resets the debounce timer. When it fires (after 200ms of quiet),
// a rulesFileChangedMsg is pushed to the channel.
func (rw *RulesWatcher) debounce() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.closed {
		return
	}

	if rw.debounceTimer != nil {
		rw.debounceTimer.Stop()
	}

	rw.debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
		rw.mu.Lock()
		closed := rw.closed
		rw.mu.Unlock()
		if closed {
			return
		}
		// Non-blocking send — if the channel already has a pending
		// message we don't need to queue another one.
		select {
		case rw.ch <- rulesFileChangedMsg{}:
		default:
		}
	})
}
