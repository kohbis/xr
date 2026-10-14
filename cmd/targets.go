package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/worktree"
	"github.com/spf13/cobra"
)

// registerWorktreeFlag adds --worktree to a cross-repository command. The
// value is a branch glob, the same filter 'xr worktree list -b' takes, so a
// task spread over several repositories is addressed by its branch names
// rather than by a recorded group.
func registerWorktreeFlag(cmd *cobra.Command, target *string, verb string) {
	cmd.Flags().StringVar(target, "worktree", "", verb+" in the worktrees whose branch matches this glob instead of the workspace checkouts")
}

// resolveTargets decides which directories a cross-repository command runs
// in. Without a worktree pattern it is the workspace: the config as loaded,
// its workspace directory, and the --repo filter for the internal package to
// apply. With one, it is the worktrees of the selected repositories whose
// branch matches the pattern.
//
// Worktrees are handed over as a view of the config, so runner, search and
// diff need no second code path: each matching worktree becomes a repository
// entry named "<repo>@<branch>" — the pair that identifies a worktree — whose
// Path is the absolute directory git reports. The workspace directory is then
// empty, since filepath.Join ignores it, and the repo filter is already
// applied. The view is for selection only and must never be saved.
//
// Worktrees whose directory is gone (prunable) are left out: there is nothing
// to run in, and 'xr worktree prune' is where they are dealt with.
func resolveTargets(cfg *config.Config, repoFilter []string, worktreePattern string) (*config.Config, string, []string, error) {
	if worktreePattern == "" {
		wsDir, err := cfg.WorkspaceDir()
		if err != nil {
			return nil, "", nil, err
		}
		return cfg, wsDir, repoFilter, nil
	}
	if strings.TrimSpace(worktreePattern) == "" {
		return nil, "", nil, errors.New("--worktree needs a branch pattern (use '*' for every worktree)")
	}

	m := worktree.New(cfg.Root(), cfg)
	entries, err := m.List(cfg.Select(repoFilter), worktreePattern)
	if err != nil {
		return nil, "", nil, fmt.Errorf("listing worktrees: %w", err)
	}

	byName := make(map[string]config.Repository, len(cfg.Repositories))
	for _, r := range cfg.Repositories {
		byName[r.Name] = r
	}

	view := *cfg
	view.Repositories = make([]config.Repository, 0, len(entries))
	for _, e := range entries {
		if e.Prunable {
			continue
		}
		repo := byName[e.Repo]
		view.Repositories = append(view.Repositories, config.Repository{
			Name:   worktreeTargetName(e.Repo, e.Branch),
			Path:   e.Path,
			Type:   repo.Type,
			Source: repo.Source,
			Branch: e.Branch,
			Tags:   repo.Tags,
		})
	}
	return &view, "", nil, nil
}

// worktreeTargetName names one worktree in output: the repository and the
// branch, which together identify it.
func worktreeTargetName(repo, branch string) string {
	return repo + "@" + branch
}
