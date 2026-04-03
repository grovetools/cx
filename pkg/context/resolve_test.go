package context

import (
	"testing"
	"time"
)

func TestParseExtendedDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// Standard Go durations
		{name: "hours", input: "24h", want: 24 * time.Hour},
		{name: "minutes", input: "30m", want: 30 * time.Minute},
		{name: "seconds", input: "300s", want: 300 * time.Second},

		// Extended: days
		{name: "days", input: "7d", want: 7 * 24 * time.Hour},
		{name: "fractional days", input: "1.5d", want: 36 * time.Hour},
		{name: "one day", input: "1d", want: 24 * time.Hour},

		// Extended: weeks
		{name: "weeks", input: "2w", want: 14 * 24 * time.Hour},
		{name: "fractional weeks", input: "0.5w", want: 84 * time.Hour},
		{name: "one week", input: "1w", want: 7 * 24 * time.Hour},

		// Quoted values
		{name: "quoted days", input: `"7d"`, want: 7 * 24 * time.Hour},
		{name: "quoted hours", input: `"24h"`, want: 24 * time.Hour},
		{name: "quoted weeks", input: `"2w"`, want: 14 * 24 * time.Hour},

		// Whitespace
		{name: "leading space", input: " 7d", want: 7 * 24 * time.Hour},
		{name: "trailing space", input: "7d ", want: 7 * 24 * time.Hour},

		// Errors
		{name: "invalid string", input: "xyz", wantErr: true},
		{name: "bare d", input: "d", wantErr: true},
		{name: "bare w", input: "w", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "space before unit", input: "7 d", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExtendedDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseExtendedDuration(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseExtendedDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseExtendedDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
