package context

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		path string
		size int64
		want int
	}{
		{"go source uses code divisor", "pkg/context/cache.go", 1000, 500},
		{"json uses code divisor", "config.json", 1000, 500},
		{"yaml uses code divisor", "ci.yml", 1000, 500},
		{"shell uses code divisor", "install.sh", 1000, 500},
		{"markdown uses prose divisor", "README.md", 1000, 250},
		{"txt uses prose divisor", "notes.txt", 1000, 250},
		{"no extension uses prose divisor", "Makefile", 1000, 250},
		{"unknown extension uses middle divisor", "data.xyz", 999, 333},
		{"extension match is case-insensitive", "Main.GO", 1000, 500},
		{"zero size", "empty.go", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateTokens(tt.path, tt.size); got != tt.want {
				t.Errorf("EstimateTokens(%q, %d) = %d, want %d", tt.path, tt.size, got, tt.want)
			}
		})
	}
}
