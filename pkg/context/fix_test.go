package context

import (
	"os"
	"testing"
)

func TestManager_FixContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir("..") }()

	mgr := NewManager(tempDir)

	// Create .grove directory
	_ = os.MkdirAll(GroveDir, 0o755)

	// FixContext is deprecated and just prints a message
	err := mgr.FixContext()
	if err != nil {
		t.Fatalf("Failed to call FixContext: %v", err)
	}

	// The function should succeed but not do anything
	// Just verify it doesn't return an error
}
