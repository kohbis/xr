package workspace

import (
	"testing"
)

func TestNormalizeGitignoreLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"repos/", "repos/"},
		{"./repos/", "repos/"},
		{"/repos/", "repos/"},
		{"./repos", "repos"},
		{"node_modules", "node_modules"},
	}

	for _, tt := range tests {
		if got := normalizeGitignoreLine(tt.input); got != tt.want {
			t.Errorf("normalizeGitignoreLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContainsLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    string
		want    bool
	}{
		{"exact match", "repos/\nnode_modules/\n", "repos/", true},
		{"not present", "repos/\n", "other/", false},
		{"normalized ./", "./repos/\n", "repos/", true},
		{"normalized /", "/repos/\n", "repos/", true},
		{"line with ./ matches plain", "repos/\n", "./repos/", true},
		{"empty content", "", "repos/", false},
		{"whitespace trimmed", "  repos/  \n", "repos/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsLine(tt.content, tt.line); got != tt.want {
				t.Errorf("containsLine(%q, %q) = %v, want %v", tt.content, tt.line, got, tt.want)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	result := expandTilde("~/projects/repo")
	if result == "~/projects/repo" {
		t.Error("expandTilde did not expand ~/")
	}
	if len(result) == 0 {
		t.Error("expandTilde returned empty string")
	}

	plain := "/absolute/path"
	if got := expandTilde(plain); got != plain {
		t.Errorf("expandTilde(%q) = %q, want unchanged", plain, got)
	}

	relative := "relative/path"
	if got := expandTilde(relative); got != relative {
		t.Errorf("expandTilde(%q) = %q, want unchanged", relative, got)
	}
}
