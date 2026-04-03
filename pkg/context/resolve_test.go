package context

import (
	"testing"
)

func TestIsRelativeExternalPath(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		// Should return true — paths that traverse outside current directory
		{"..", true},
		{"../foo", true},
		{"../foo/bar", true},
		{"./../foo", true},       // uncleaned ./..
		{"././../foo", true},     // multiple ./ prefixes
		{"../../foo", true},      // multiple parent traversals
		{"../", true},            // trailing slash
		{"./../", true},          // uncleaned with trailing slash

		// Should return false — paths that stay within current directory
		{".", false},
		{"./foo", false},
		{"foo", false},
		{"foo/../bar", false},    // traversal stays within subtree
		{"foo/bar", false},
		{"/absolute/path", false},
		{"", false},              // empty string
		{"..foo", false},         // not a traversal, just a name starting with ..
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := IsRelativeExternalPath(tt.pattern)
			if got != tt.want {
				t.Errorf("IsRelativeExternalPath(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}
