// Package worktree manages git worktrees for the repositories of a workspace.
//
// A worktree is identified by the pair (repository, branch): git itself refuses
// to check out the same branch in two worktrees, so that pair is the smallest
// unit that can exist. Nothing about worktrees is persisted in repos.yaml —
// `git worktree list` is the single source of truth, which keeps xr in step
// with worktrees created or deleted outside of it.
//
// Worktrees are placed at <worktrees dir>/<repo path>/<branch>. Branch names
// containing slashes nest as directories, mirroring git's own ref namespace.
package worktree

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
)

// Manager performs worktree operations across the repositories of a workspace.
type Manager struct {
	Config *config.Config
	// Root is the directory containing repos.yaml; workspace and worktree
	// directories are resolved relative to it.
	Root string
}

func New(root string, cfg *config.Config) *Manager {
	return &Manager{Root: root, Config: cfg}
}

// WorktreesDir returns the directory holding all worktrees of the workspace.
func (m *Manager) WorktreesDir() string {
	return filepath.Join(m.Root, m.Config.Worktrees)
}

func (m *Manager) reposDir() string {
	return filepath.Join(m.Root, m.Config.Workspace)
}

// RepoDir returns the git directory backing repo, resolving symlink repos to
// their real location.
func (m *Manager) RepoDir(repo config.Repository) (string, error) {
	path := filepath.Join(m.reposDir(), repo.Path)
	if _, err := os.Lstat(path); err != nil {
		return "", fmt.Errorf("missing in workspace (run 'xr init'): %s", path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	if _, err := os.Stat(filepath.Join(resolved, ".git")); err != nil {
		return "", fmt.Errorf("not a git repository: %s", resolved)
	}
	return resolved, nil
}

// PathFor returns the worktree directory for repo and branch.
func (m *Manager) PathFor(repo config.Repository, branch string) string {
	return filepath.Join(m.WorktreesDir(), repo.Path, filepath.FromSlash(branch))
}

// SelectRepos resolves repository names against the config. An empty names
// slice selects every repository.
func (m *Manager) SelectRepos(names []string) ([]config.Repository, error) {
	if len(names) == 0 {
		return slices.Clone(m.Config.Repositories), nil
	}
	byName := make(map[string]config.Repository, len(m.Config.Repositories))
	for _, r := range m.Config.Repositories {
		byName[r.Name] = r
	}
	repos := make([]config.Repository, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		r, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("repository %q not found in config", name)
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		repos = append(repos, r)
	}
	return repos, nil
}

// Entry is a linked worktree of one repository.
type Entry struct {
	Repo     string `json:"repo"`
	Branch   string `json:"branch,omitempty"`
	Path     string `json:"path"`
	Head     string `json:"head,omitempty"`
	Detached bool   `json:"detached,omitempty"`
	Locked   bool   `json:"locked,omitempty"`
	Prunable bool   `json:"prunable,omitempty"`
	Status   string `json:"status,omitempty"`
}

// List returns the linked worktrees of the given repositories. The main
// worktree of each repository is excluded — that is the checkout managed by
// `xr repo sync`. When branchPattern is non-empty it filters branches by glob.
func (m *Manager) List(repos []config.Repository, branchPattern string) ([]Entry, error) {
	if err := validatePattern(branchPattern); err != nil {
		return nil, err
	}

	var entries []Entry
	for _, repo := range repos {
		dir, err := m.RepoDir(repo)
		if err != nil {
			continue
		}
		worktrees, err := git.Worktrees(dir)
		if err != nil {
			return nil, fmt.Errorf("listing worktrees of %s: %w", repo.Name, err)
		}
		for _, wt := range worktrees {
			if wt.Main || wt.Bare {
				continue
			}
			ok, err := matchBranch(branchPattern, wt.Branch)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			entries = append(entries, Entry{
				Repo:     repo.Name,
				Branch:   wt.Branch,
				Path:     wt.Path,
				Head:     wt.Head,
				Detached: wt.Detached,
				Locked:   wt.Locked,
				Prunable: wt.Prunable,
				Status:   worktreeStatus(wt),
			})
		}
	}
	return entries, nil
}

func worktreeStatus(wt git.Worktree) string {
	if wt.Prunable {
		return "!"
	}
	snapshot, err := git.Inspect(wt.Path)
	if err != nil {
		return "!"
	}
	return snapshot.Status
}

// validatePattern rejects a malformed glob up front, so that a bad pattern is
// reported even when no worktree exists to match it against.
func validatePattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return fmt.Errorf("invalid branch pattern %q: %w", pattern, err)
	}
	return nil
}

// matchBranch reports whether branch satisfies pattern. An empty pattern
// matches everything; a non-empty pattern never matches a detached worktree.
func matchBranch(pattern, branch string) (bool, error) {
	if pattern == "" {
		return true, nil
	}
	if branch == "" {
		return false, nil
	}
	// Branch names are always slash-separated, so match with path.Match rather
	// than filepath.Match, whose separator is platform-dependent.
	ok, err := path.Match(pattern, branch)
	if err != nil {
		return false, fmt.Errorf("invalid branch pattern %q: %w", pattern, err)
	}
	return ok, nil
}

// Outcome records what happened to one repository during an operation.
type Outcome struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"` // created, removed, pruned, skipped, preview, failed
	Detail string `json:"detail,omitempty"`
}

const (
	StatusCreated = "created"
	StatusRemoved = "removed"
	StatusPruned  = "pruned"
	StatusSkipped = "skipped"
	StatusPreview = "preview"
	StatusFailed  = "failed"
)

// Result aggregates the outcomes of an operation.
type Result struct {
	Outcomes []Outcome `json:"outcomes"`
}

// Counts summarizes outcomes into changed/skipped/failed totals. Previews count
// as skipped, since nothing was written.
func (r *Result) Counts() (changed, skipped, failed int) {
	for _, o := range r.Outcomes {
		switch o.Status {
		case StatusCreated, StatusRemoved, StatusPruned:
			changed++
		case StatusFailed:
			failed++
		default:
			skipped++
		}
	}
	return changed, skipped, failed
}

// AddOptions configures Add.
type AddOptions struct {
	// Create allows creating the branch when it exists neither locally nor on
	// origin. Without it, a missing branch is an error.
	Create bool
	// Base is the start point for a branch created by Create. Empty falls back
	// to the branch configured for the repository in repos.yaml, then HEAD.
	Base   string
	DryRun bool
}

// Add creates a worktree for branch in each of the given repositories.
func (m *Manager) Add(branch string, repos []config.Repository, opts AddOptions) (*Result, error) {
	if err := validateBranch(branch); err != nil {
		return nil, err
	}

	result := &Result{}
	for _, repo := range repos {
		outcome := m.addOne(branch, repo, opts)
		result.Outcomes = append(result.Outcomes, outcome)
	}
	return result, nil
}

func (m *Manager) addOne(branch string, repo config.Repository, opts AddOptions) Outcome {
	outcome := Outcome{Repo: repo.Name, Branch: branch}

	dir, err := m.RepoDir(repo)
	if err != nil {
		outcome.Status = StatusFailed
		outcome.Detail = err.Error()
		return outcome
	}

	target := m.PathFor(repo, branch)
	outcome.Path = target
	if err := validateInsideDir(m.WorktreesDir(), target); err != nil {
		outcome.Status = StatusFailed
		outcome.Detail = err.Error()
		return outcome
	}

	if _, err := os.Lstat(target); err == nil {
		outcome.Status = StatusSkipped
		outcome.Detail = "worktree directory already exists"
		return outcome
	}

	addOpts, detail, err := m.resolveBranchSource(dir, repo, branch, opts)
	if err != nil {
		outcome.Status = StatusFailed
		outcome.Detail = err.Error()
		return outcome
	}

	if opts.DryRun {
		outcome.Status = StatusPreview
		outcome.Detail = detail
		return outcome
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		outcome.Status = StatusFailed
		outcome.Detail = fmt.Sprintf("creating parent directory: %v", err)
		return outcome
	}
	if err := git.WorktreeAdd(dir, target, branch, addOpts); err != nil {
		outcome.Status = StatusFailed
		outcome.Detail = err.Error()
		removeEmptyDirs(m.WorktreesDir(), filepath.Dir(target))
		return outcome
	}

	outcome.Status = StatusCreated
	outcome.Detail = detail
	return outcome
}

// resolveBranchSource decides how the worktree's branch is obtained: checking
// out an existing local branch, branching off origin, or creating a new branch.
func (m *Manager) resolveBranchSource(dir string, repo config.Repository, branch string, opts AddOptions) (git.WorktreeAddOptions, string, error) {
	localExists, err := git.RefExists(dir, "refs/heads/"+branch)
	if err != nil {
		return git.WorktreeAddOptions{}, "", fmt.Errorf("checking local branch: %w", err)
	}
	if localExists {
		return git.WorktreeAddOptions{}, "checkout existing branch", nil
	}

	remoteRef := "origin/" + branch
	remoteExists, err := git.RefExists(dir, "refs/remotes/"+remoteRef)
	if err != nil {
		return git.WorktreeAddOptions{}, "", fmt.Errorf("checking remote branch: %w", err)
	}
	if remoteExists {
		return git.WorktreeAddOptions{CreateBranch: true, Track: true, Base: remoteRef},
			fmt.Sprintf("track %s", remoteRef), nil
	}

	if !opts.Create {
		return git.WorktreeAddOptions{}, "", fmt.Errorf("branch %q not found locally or on origin (use --create)", branch)
	}

	base := opts.Base
	if base == "" {
		base = repo.Branch
	}
	if base == "" {
		base = "HEAD"
	}
	baseExists, err := git.RefExists(dir, base)
	if err != nil {
		return git.WorktreeAddOptions{}, "", fmt.Errorf("checking base ref: %w", err)
	}
	if !baseExists {
		return git.WorktreeAddOptions{}, "", fmt.Errorf("base ref %q not found", base)
	}
	return git.WorktreeAddOptions{CreateBranch: true, Base: base},
		fmt.Sprintf("create branch from %s", base), nil
}

// Remove deletes the given worktrees. Unless force is set, git refuses to
// remove a worktree with uncommitted changes.
func (m *Manager) Remove(entries []Entry, force, dryRun bool) (*Result, error) {
	dirByRepo := make(map[string]string, len(m.Config.Repositories))
	for _, repo := range m.Config.Repositories {
		dir, err := m.RepoDir(repo)
		if err != nil {
			continue
		}
		dirByRepo[repo.Name] = dir
	}

	result := &Result{}
	for _, entry := range entries {
		outcome := Outcome{Repo: entry.Repo, Branch: entry.Branch, Path: entry.Path}
		dir, ok := dirByRepo[entry.Repo]
		if !ok {
			outcome.Status = StatusFailed
			outcome.Detail = "repository not available in workspace"
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}

		if dryRun {
			outcome.Status = StatusPreview
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}

		if err := git.WorktreeRemove(dir, entry.Path, force); err != nil {
			outcome.Status = StatusFailed
			outcome.Detail = err.Error()
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		removeEmptyDirs(m.WorktreesDir(), filepath.Dir(entry.Path))

		outcome.Status = StatusRemoved
		result.Outcomes = append(result.Outcomes, outcome)
	}
	return result, nil
}

// Prune drops administrative entries for worktrees whose directory has been
// deleted outside of git.
func (m *Manager) Prune(repos []config.Repository, dryRun bool) (*Result, error) {
	result := &Result{}
	for _, repo := range repos {
		outcome := Outcome{Repo: repo.Name}
		dir, err := m.RepoDir(repo)
		if err != nil {
			outcome.Status = StatusSkipped
			outcome.Detail = err.Error()
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		report, err := git.WorktreePrune(dir, dryRun)
		if err != nil {
			outcome.Status = StatusFailed
			outcome.Detail = err.Error()
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		if report == "" {
			outcome.Status = StatusSkipped
			outcome.Detail = "nothing to prune"
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		outcome.Detail = fmt.Sprintf("%d stale entry(ies)", len(strings.Split(report, "\n")))
		if dryRun {
			outcome.Status = StatusPreview
		} else {
			outcome.Status = StatusPruned
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}
	return result, nil
}

// GoneEntries returns worktrees whose branch tracked a remote branch that no
// longer exists, i.e. the typical leftovers of a merged and deleted pull
// request. It reflects the last fetch, so run `xr repo sync --update --prune`
// first for an accurate answer.
func (m *Manager) GoneEntries(repos []config.Repository) ([]Entry, error) {
	entries, err := m.List(repos, "")
	if err != nil {
		return nil, err
	}

	dirByRepo := make(map[string]string, len(repos))
	for _, repo := range repos {
		dir, err := m.RepoDir(repo)
		if err != nil {
			continue
		}
		dirByRepo[repo.Name] = dir
	}

	var gone []Entry
	for _, entry := range entries {
		if entry.Branch == "" {
			continue
		}
		dir, ok := dirByRepo[entry.Repo]
		if !ok {
			continue
		}
		isGone, err := git.UpstreamGone(dir, entry.Branch)
		if err != nil {
			return nil, fmt.Errorf("checking upstream of %s/%s: %w", entry.Repo, entry.Branch, err)
		}
		if isGone {
			gone = append(gone, entry)
		}
	}
	return gone, nil
}

func validateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return errors.New("branch name is required")
	}
	if filepath.IsAbs(branch) || strings.HasPrefix(branch, "~") {
		return fmt.Errorf("invalid branch name %q", branch)
	}
	// A leading dash would reach git as an option rather than a branch name.
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("invalid branch name %q", branch)
	}
	for _, part := range strings.Split(filepath.ToSlash(branch), "/") {
		if part == "." || part == ".." || part == "" {
			return fmt.Errorf("invalid branch name %q", branch)
		}
	}
	return nil
}

// validateInsideDir ensures path is contained within dir.
func validateInsideDir(dir, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes worktree directory", path)
	}
	return nil
}

// resolvePath makes p absolute and resolves symlinks when the path exists.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// removeEmptyDirs deletes empty directories from dir upwards, stopping at (and
// keeping) root. It cleans up the intermediate directories of nested branch
// names such as feature/foo.
func removeEmptyDirs(root, dir string) {
	// git reports worktree paths with symlinks resolved, so resolve both sides
	// before comparing them.
	absRoot, err := resolvePath(root)
	if err != nil {
		return
	}
	current, err := resolvePath(dir)
	if err != nil {
		return
	}
	for current != absRoot {
		if err := validateInsideDir(absRoot, current); err != nil {
			return
		}
		if err := os.Remove(current); err != nil {
			return // not empty, or gone already
		}
		current = filepath.Dir(current)
	}
}
