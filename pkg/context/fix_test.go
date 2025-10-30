package context

import (
	"os"
	"testing"
)

func TestManager_FixContext(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir("..")

	mgr := NewManager(tempDir)

	// Create .grove directory
	os.MkdirAll(GroveDir, 0755)

	// FixContext is deprecated and just prints a message
	err := mgr.FixContext()
	if err != nil {
		t.Fatalf("Failed to call FixContext: %v", err)
	}

	// The function should succeed but not do anything
	// Just verify it doesn't return an error
}
