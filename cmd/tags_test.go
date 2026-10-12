package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kohbis/xr/internal/output"
)

func TestExecTagAndRepoAreAND(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `repositories:
  - name: api
    source: https://example.com/api.git
    tags: [backend, go]
  - name: web
    source: https://example.com/web.git
    tags: [frontend]
  - name: lib
    source: https://example.com/lib.git
    tags: [backend]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		execRepo = nil
		execTags = nil
		execJSON = false
		if err := rootCmd.PersistentFlags().Set("config", ""); err != nil {
			t.Error(err)
		}
	})
	if err := rootCmd.PersistentFlags().Set("config", cfgPath); err != nil {
		t.Fatal(err)
	}
	execJSON = true
	execJobs = 1

	tests := []struct {
		name  string
		tags  []string
		repos []string
		want  []string
	}{
		{
			name:  "tag and repo",
			tags:  []string{"backend"},
			repos: []string{"web", "api"},
			want:  []string{"api"},
		},
		{
			name:  "named repo without the tag",
			tags:  []string{"frontend"},
			repos: []string{"api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execTags = tt.tags
			execRepo = tt.repos

			out, err := captureStdout(t, func() error {
				return execCmd.RunE(execCmd, []string{"true"})
			})
			if err != nil {
				t.Fatalf("exec: %v\n%s", err, out)
			}

			var result output.CommandResult
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			var got []string
			for _, repo := range result.Repos {
				got = append(got, repo.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("repos = %v, want %v", got, tt.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), runErr
}
