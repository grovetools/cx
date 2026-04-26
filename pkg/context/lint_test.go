package context

import (
	"testing"
)

func TestFormatLintIssue(t *testing.T) {
	tests := []struct {
		issue LintIssue
		want  string
	}{
		{
			issue: LintIssue{LineNum: 5, Severity: "Error", Message: "traversal outside workspace"},
			want:  "[Error] Line 5: traversal outside workspace",
		},
		{
			issue: LintIssue{LineNum: 12, Severity: "Warning", Message: "pattern matches 0 files"},
			want:  "[Warning] Line 12: pattern matches 0 files",
		},
		{
			issue: LintIssue{LineNum: 1, Severity: "Notice", Message: "unused directive"},
			want:  "[Notice] Line 1: unused directive",
		},
	}
	for _, tt := range tests {
		got := FormatLintIssue(tt.issue)
		if got != tt.want {
			t.Errorf("FormatLintIssue(%+v) = %q, want %q", tt.issue, got, tt.want)
		}
	}
}

func TestHighestSeverity(t *testing.T) {
	tests := []struct {
		name   string
		issues []LintIssue
		want   string
	}{
		{
			name:   "empty",
			issues: nil,
			want:   "",
		},
		{
			name:   "single error",
			issues: []LintIssue{{Severity: "Error"}},
			want:   "Error",
		},
		{
			name:   "single warning",
			issues: []LintIssue{{Severity: "Warning"}},
			want:   "Warning",
		},
		{
			name:   "single notice",
			issues: []LintIssue{{Severity: "Notice"}},
			want:   "Notice",
		},
		{
			name:   "warning + error = error",
			issues: []LintIssue{{Severity: "Warning"}, {Severity: "Error"}},
			want:   "Error",
		},
		{
			name:   "notice + warning = warning",
			issues: []LintIssue{{Severity: "Notice"}, {Severity: "Warning"}},
			want:   "Warning",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HighestSeverity(tt.issues)
			if got != tt.want {
				t.Errorf("HighestSeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLintIssuesByLine(t *testing.T) {
	issues := []LintIssue{
		{LineNum: 1, Severity: "Warning", Message: "a"},
		{LineNum: 1, Severity: "Error", Message: "b"},
		{LineNum: 5, Severity: "Warning", Message: "c"},
	}
	byLine := LintIssuesByLine(issues)
	if len(byLine[1]) != 2 {
		t.Errorf("expected 2 issues on line 1, got %d", len(byLine[1]))
	}
	if len(byLine[5]) != 1 {
		t.Errorf("expected 1 issue on line 5, got %d", len(byLine[5]))
	}
}
