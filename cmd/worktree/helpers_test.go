package worktree

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kohbis/xr/internal/config"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

func newTestManager(t *testing.T) *wt.Manager {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.yaml")
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return wt.New(cfg.Root(), cfg)
}

func repoNames(repos []config.Repository) []string {
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names
}

func TestSelectRepos(t *testing.T) {
	m := newTestManager(t)

	tests := []struct {
		name    string
		repos   []string
		tags    []string
		want    []string
		wantErr bool
	}{
		{name: "no filter", want: []string{"api", "web", "lib"}},
		{name: "tag only", tags: []string{"backend"}, want: []string{"api", "lib"}},
		{name: "any of several tags", tags: []string{"go", "frontend"}, want: []string{"api", "web"}},
		{name: "repo and tag must both match", repos: []string{"web", "api"}, tags: []string{"backend"}, want: []string{"api"}},
		{name: "named repo without the tag", repos: []string{"web"}, tags: []string{"backend"}, want: []string{}},
		{name: "unknown tag", tags: []string{"nope"}, want: []string{}},
		{name: "unknown repo is an error even with a tag", repos: []string{"typo"}, tags: []string{"backend"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectRepos(m, tt.repos, tt.tags)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", repoNames(got))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if names := repoNames(got); !slices.Equal(names, tt.want) {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

// A tag selects repositories by itself, so add must not fall back to the
// prompt (which would fail here: the bare command has no interactive flags).
func TestResolveTargetRepos_TagSkipsPrompt(t *testing.T) {
	m := newTestManager(t)
	got, err := resolveTargetRepos(&cobra.Command{}, m, nil, []string{"backend"})
	if err != nil {
		t.Fatal(err)
	}
	if names := repoNames(got); !slices.Equal(names, []string{"api", "lib"}) {
		t.Errorf("got %v, want [api lib]", names)
	}
}
