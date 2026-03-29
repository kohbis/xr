package shellcomp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoNameCandidates(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "repos.yaml")
	content := `workspace: ./repos
repositories:
  - name: alpha
    source: https://example.com/a.git
  - name: beta
    source: https://example.com/b.git
  - name: alphonso
    source: https://example.com/c.git
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("prefix match", func(t *testing.T) {
		got := RepoNameCandidates(cfgPath, nil, "al")
		want := []string{"alpha", "alphonso"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("exclude args", func(t *testing.T) {
		got := RepoNameCandidates(cfgPath, []string{"alpha"}, "al")
		want := []string{"alphonso"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		got := RepoNameCandidates(filepath.Join(dir, "nope.yaml"), nil, "")
		if len(got) != 0 {
			t.Fatalf("got %v, want empty", got)
		}
	})
}
