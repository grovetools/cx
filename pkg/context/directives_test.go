package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSearchDirectives(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantBase          string
		wantDirectives    []SearchDirective
		wantHasDirectives bool
	}{
		{
			name:  "single find directive",
			input: `pkg/**/*.go @find: "api"`,
			wantBase:          "pkg/**/*.go",
			wantDirectives:    []SearchDirective{{Name: "find", Query: "api"}},
			wantHasDirectives: true,
		},
		{
			name:  "single grep directive",
			input: `pkg/**/*.go @grep: "User"`,
			wantBase:          "pkg/**/*.go",
			wantDirectives:    []SearchDirective{{Name: "grep", Query: "User"}},
			wantHasDirectives: true,
		},
		{
			name:  "combined find and grep",
			input: `pkg/**/*.go @find: "api" @grep: "User"`,
			wantBase: "pkg/**/*.go",
			wantDirectives: []SearchDirective{
				{Name: "find", Query: "api"},
				{Name: "grep", Query: "User"},
			},
			wantHasDirectives: true,
		},
		{
			name:  "multiple same type",
			input: `pkg/**/*.go @find: "api" @find: "v1"`,
			wantBase: "pkg/**/*.go",
			wantDirectives: []SearchDirective{
				{Name: "find", Query: "api"},
				{Name: "find", Query: "v1"},
			},
			wantHasDirectives: true,
		},
		{
			name:              "no directives",
			input:             "pkg/**/*.go",
			wantBase:          "pkg/**/*.go",
			wantDirectives:    nil,
			wantHasDirectives: false,
		},
		{
			name:              "malformed unclosed quote",
			input:             `pkg/**/*.go @find: "unclosed`,
			wantBase:          "pkg/**/*.go",
			wantDirectives:    nil,
			wantHasDirectives: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, directives, hasDirectives := parseSearchDirectives(tt.input)
			assert.Equal(t, tt.wantBase, base)
			assert.Equal(t, tt.wantDirectives, directives)
			assert.Equal(t, tt.wantHasDirectives, hasDirectives)
		})
	}
}

func TestEncodeDecodeDirectivesRoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		pattern        string
		directives     []SearchDirective
		wantBase       string
		wantDirectives []SearchDirective
		wantHas        bool
	}{
		{
			name:    "multiple directives",
			pattern: "pkg/**/*.go",
			directives: []SearchDirective{
				{Name: "find", Query: "api"},
				{Name: "grep", Query: "User"},
			},
			wantBase: "pkg/**/*.go",
			wantDirectives: []SearchDirective{
				{Name: "find", Query: "api"},
				{Name: "grep", Query: "User"},
			},
			wantHas: true,
		},
		{
			name:    "single directive",
			pattern: "src/**/*.ts",
			directives: []SearchDirective{
				{Name: "grep", Query: "Handler"},
			},
			wantBase: "src/**/*.ts",
			wantDirectives: []SearchDirective{
				{Name: "grep", Query: "Handler"},
			},
			wantHas: true,
		},
		{
			name:           "no directives",
			pattern:        "pkg/**/*.go",
			directives:     []SearchDirective{},
			wantBase:       "pkg/**/*.go",
			wantDirectives: nil,
			wantHas:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeDirectives(tt.pattern, tt.directives)
			base, directives, has := decodeDirectives(encoded)
			assert.Equal(t, tt.wantBase, base)
			assert.Equal(t, tt.wantDirectives, directives)
			assert.Equal(t, tt.wantHas, has)
		})
	}
}
