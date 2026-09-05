package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/interactive"
	"github.com/kohbis/xr/internal/shellcomp"
	wt "github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

func newManager(cmd *cobra.Command) (*wt.Manager, error) {
	cfg, err := config.LoadCommand(cmd)
	if err != nil {
		return nil, err
	}
	return wt.New(cfg.Root(), cfg), nil
}

// resolveTargetRepos turns --repo values into repositories. When none are given
// it prompts for a selection, since worktrees are usually wanted for a subset of
// the workspace rather than all of it.
func resolveTargetRepos(cmd *cobra.Command, m *wt.Manager, names []string) ([]config.Repository, error) {
	if len(names) > 0 {
		return m.SelectRepos(names)
	}

	shouldPrompt, err := interactive.ShouldPrompt(cmd)
	if err != nil {
		return nil, err
	}
	if !shouldPrompt {
		return nil, fmt.Errorf("missing required value(s): --repo (non-interactive)")
	}

	all := m.Config.Repositories
	if len(all) == 0 {
		return nil, fmt.Errorf("no repositories found in config")
	}
	candidates := make([]string, 0, len(all))
	for _, r := range all {
		candidates = append(candidates, r.Name)
	}
	selected, err := interactive.MultiSelectByDone("Select repos for the worktree (search enabled)", candidates, 15)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, nil
	}
	return m.SelectRepos(selected)
}

// displayPath shortens a path relative to the working directory when that is
// shorter than the absolute form.
func displayPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

func completeRepoFlag(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	names := shellcomp.RepoNameCandidates(config.CommandPath(cmd), nil, toComplete)
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeBranches(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	m, err := newManager(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := m.List(m.Config.Repositories, "")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	seen := make(map[string]struct{}, len(entries))
	var branches []string
	for _, e := range entries {
		if e.Branch == "" || !strings.HasPrefix(e.Branch, toComplete) {
			continue
		}
		if _, ok := seen[e.Branch]; ok {
			continue
		}
		seen[e.Branch] = struct{}{}
		branches = append(branches, e.Branch)
	}
	return branches, cobra.ShellCompDirectiveNoFileComp
}
