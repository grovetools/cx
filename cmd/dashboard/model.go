package dashboard

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattsolo1/grove-context/pkg/context"
	"github.com/mattsolo1/grove-core/tui/components/help"
)

// dashboardModel is the model for the interactive dashboard
type dashboardModel struct {
	hotStats    *context.ContextStats
	coldStats   *context.ContextStats
	watcher     *fsnotify.Watcher
	width       int
	height      int
	err         error
	lastUpdate  time.Time
	isUpdating  bool
	horizontal  bool
	help        help.Model
}

// newDashboardModel creates a new dashboard model
func newDashboardModel(horizontal bool) (*dashboardModel, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the current directory recursively
	if err := watchDirectory(".", watcher); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	helpModel := help.NewBuilder().
		WithKeys(dashboardKeys).
		WithTitle("Grove Context Dashboard - Help").
		Build()

	return &dashboardModel{
		watcher:    watcher,
		lastUpdate: time.Now(),
		horizontal: horizontal,
		help:       helpModel,
	}, nil
}
