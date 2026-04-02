package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
)

type Workspace struct {
	Config *config.Config
	Root   string
}

func New(root string, cfg *config.Config) *Workspace {
	return &Workspace{Root: root, Config: cfg}
}

func (w *Workspace) Init() error {
	if err := os.MkdirAll(w.Root, 0755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	if err := w.gitInit(); err != nil {
		return err
	}

	wsDir := filepath.Join(w.Root, w.Config.Workspace)
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return fmt.Errorf("creating repos directory: %w", err)
	}

	if len(w.Config.Repositories) == 0 {
		if err := w.createReadme(); err != nil {
			return fmt.Errorf("creating README: %w", err)
		}
		return nil
	}

	for _, repo := range w.Config.Repositories {
		if err := w.addRepo(repo, wsDir); err != nil {
			return fmt.Errorf("adding repo %s: %w", repo.Name, err)
		}
	}

	return nil
}

func (w *Workspace) CreateGitignore(ignoreWorkspace bool) error {
	gitignorePath := filepath.Join(w.Root, ".gitignore")

	existing, _ := os.ReadFile(gitignorePath)
	entry := strings.TrimPrefix(w.Config.Workspace, "./") + "/"

	if ignoreWorkspace {
		if containsLine(string(existing), entry) {
			fmt.Printf("  %s is already in .gitignore\n", entry)
			return nil
		}
		content := string(existing)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry + "\n"
		fmt.Printf("  adding %s to .gitignore\n", entry)
		return os.WriteFile(gitignorePath, []byte(content), 0644)
	}

	fmt.Println("  .gitignore unchanged")
	return nil
}

func containsLine(content, line string) bool {
	normalized := normalizeGitignoreLine(line)
	for _, l := range strings.Split(content, "\n") {
		if normalizeGitignoreLine(strings.TrimSpace(l)) == normalized {
			return true
		}
	}
	return false
}

func normalizeGitignoreLine(s string) string {
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "/")
	return s
}

func (w *Workspace) createReadme() error {
	readmePath := filepath.Join(w.Root, "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil // already exists
	}
	content := "# Workspace\n\nInitialized by xr. Edit `repos.yaml` to add repositories, then run `xr init`.\n"
	fmt.Printf("  creating README.md\n")
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func (w *Workspace) gitInit() error {
	gitDir := filepath.Join(w.Root, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return nil // already initialized
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func (w *Workspace) addRepo(repo config.Repository, wsDir string) error {
	destPath := filepath.Join(wsDir, repo.Path)
	if repo.IsSymlink() {
		return w.addSymlink(repo, destPath)
	}
	if repo.IsClone() {
		return w.addClone(repo, destPath)
	}
	return w.addSubmodule(repo, destPath)
}

// Add adds a single repository to the workspace.
func (w *Workspace) Add(repo config.Repository) error {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}
	return w.addRepo(repo, wsDir)
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (w *Workspace) addSymlink(repo config.Repository, destPath string) error {
	if _, err := os.Lstat(destPath); err == nil {
		fmt.Printf("  symlink %s already exists, skipping\n", repo.Name)
		return nil
	}
	source := expandTilde(repo.Source)
	fmt.Printf("  creating symlink %s -> %s\n", repo.Name, source)
	if err := os.Symlink(source, destPath); err != nil {
		return fmt.Errorf("creating symlink: %w", err)
	}
	return nil
}

func (w *Workspace) addSubmodule(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  submodule %s already exists, skipping\n", repo.Name)
		return nil
	}

	relPath, err := filepath.Rel(w.Root, destPath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	fmt.Printf("  adding submodule %s from %s\n", repo.Name, repo.Source)

	// -f is required because the workspace directory may be listed in .gitignore
	// (added by `xr gitignore`), which would otherwise prevent git submodule add.
	args := []string{"submodule", "add", "-f"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.Source, relPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git submodule add: %w", err)
	}
	return nil
}

func (w *Workspace) addClone(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  clone %s already exists, skipping\n", repo.Name)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	fmt.Printf("  cloning %s from %s\n", repo.Name, repo.Source)

	args := []string{"clone"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.Source, destPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	return nil
}

func (w *Workspace) Update(repoNames []string, pull bool) error {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)

	for _, repo := range w.Config.Repositories {
		if len(repoNames) > 0 && !slices.Contains(repoNames, repo.Name) {
			continue
		}

		destPath := filepath.Join(wsDir, repo.Path)
		var err error
		switch {
		case repo.IsSymlink():
			err = w.updateSymlink(repo, destPath)
		case repo.IsClone():
			err = w.updateClone(repo, destPath, pull)
		default:
			err = w.updateSubmodule(repo, destPath, pull)
		}
		if err != nil {
			fmt.Printf("  warning: %s: %v\n", repo.Name, err)
		}
	}

	return nil
}

func (w *Workspace) updateSymlink(repo config.Repository, destPath string) error {
	info, err := os.Lstat(destPath)
	if err != nil {
		fmt.Printf("  symlink %s missing, recreating\n", repo.Name)
		return w.addSymlink(repo, destPath)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists but is not a symlink", destPath)
	}
	target, err := os.Readlink(destPath)
	if err != nil {
		return fmt.Errorf("reading symlink: %w", err)
	}
	source := expandTilde(repo.Source)
	if target == source {
		fmt.Printf("  symlink %s ok\n", repo.Name)
	} else {
		fmt.Printf("  symlink %s points to %s (expected %s)\n", repo.Name, target, source)
	}
	return nil
}

func (w *Workspace) updateSubmodule(repo config.Repository, destPath string, pull bool) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return w.addSubmodule(repo, destPath)
	}

	fmt.Printf("  updating submodule %s\n", repo.Name)

	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = w.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git submodule update: %w", err)
	}

	if pull {
		fmt.Printf("  pulling %s\n", repo.Name)
		pullCmd := exec.Command("git", "pull", "origin", repo.Branch)
		pullCmd.Dir = destPath
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	}

	return nil
}

func (w *Workspace) updateClone(repo config.Repository, destPath string, pull bool) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return w.addClone(repo, destPath)
	}

	if pull {
		fmt.Printf("  pulling %s\n", repo.Name)
		args := []string{"pull", "origin"}
		if repo.Branch != "" {
			args = append(args, repo.Branch)
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = destPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	} else {
		fmt.Printf("  clone %s ok\n", repo.Name)
	}

	return nil
}

// Remove removes the given repositories from the workspace filesystem.
// The removal method depends on the repository type.
func (w *Workspace) Remove(repos []config.Repository) error {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)

	for _, repo := range repos {
		destPath := filepath.Join(wsDir, repo.Path)
		if err := validateInsideDir(wsDir, destPath); err != nil {
			return fmt.Errorf("unsafe path for %s: %w", repo.Name, err)
		}
		var err error
		switch {
		case repo.IsSymlink():
			err = w.removeSymlink(repo, destPath)
		case repo.IsClone():
			err = w.removeClone(repo, destPath)
		default:
			err = w.removeSubmodule(repo, destPath)
		}
		if err != nil {
			return fmt.Errorf("removing %s: %w", repo.Name, err)
		}
	}

	return nil
}

// validateInsideDir ensures destPath is contained within dir.
func validateInsideDir(dir, destPath string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absDir, absDest)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %q escapes workspace directory", destPath)
	}
	return nil
}

func (w *Workspace) removeSymlink(repo config.Repository, destPath string) error {
	info, err := os.Lstat(destPath)
	if os.IsNotExist(err) {
		fmt.Printf("  symlink %s already removed\n", repo.Name)
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists but is not a symlink", destPath)
	}
	fmt.Printf("  removing symlink %s\n", repo.Name)
	return os.Remove(destPath)
}

func (w *Workspace) removeClone(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		fmt.Printf("  clone %s already removed\n", repo.Name)
		return nil
	}
	fmt.Printf("  removing clone %s\n", repo.Name)
	return os.RemoveAll(destPath)
}

func (w *Workspace) removeSubmodule(repo config.Repository, destPath string) error {
	relPath, err := filepath.Rel(w.Root, destPath)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	fmt.Printf("  removing submodule %s\n", repo.Name)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		fmt.Printf("  submodule %s directory already removed, cleaning up references\n", repo.Name)
	} else {
		// git submodule deinit -f <path>
		deinit := exec.Command("git", "submodule", "deinit", "-f", relPath)
		deinit.Dir = w.Root
		deinit.Stdout = os.Stdout
		deinit.Stderr = os.Stderr
		if err := deinit.Run(); err != nil {
			fmt.Printf("  warning: git submodule deinit: %v (continuing)\n", err)
		}
	}

	// git rm -f --ignore-unmatch <path>
	rm := exec.Command("git", "rm", "-f", "--ignore-unmatch", relPath)
	rm.Dir = w.Root
	rm.Stdout = os.Stdout
	rm.Stderr = os.Stderr
	if err := rm.Run(); err != nil {
		fmt.Printf("  warning: git rm: %v (continuing)\n", err)
	}

	// Clean up .git/modules/<path> if it remains
	modulesPath := filepath.Join(w.Root, ".git", "modules", relPath)
	if _, err := os.Stat(modulesPath); err == nil {
		fmt.Printf("  cleaning up .git/modules/%s\n", relPath)
		if err := os.RemoveAll(modulesPath); err != nil {
			return fmt.Errorf("removing git modules dir: %w", err)
		}
	}

	return nil
}

// ScanRepos scans the workspace directory and detects repositories.
func (w *Workspace) ScanRepos() ([]config.Repository, error) {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return nil, fmt.Errorf("reading workspace directory: %w", err)
	}

	var repos []config.Repository
	for _, entry := range entries {
		repo, err := detectRepo(wsDir, entry)
		if err != nil {
			fmt.Printf("  warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}
		if repo == nil {
			continue
		}
		if repo.Source == "" {
			fmt.Printf("  warning: %s: no origin remote found, source will be empty\n", repo.Name)
		}
		repos = append(repos, *repo)
	}
	return repos, nil
}

func detectRepo(wsDir string, entry os.DirEntry) (*config.Repository, error) {
	name := entry.Name()
	path := filepath.Join(wsDir, name)

	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("readlink: %w", err)
		}
		return &config.Repository{
			Name:   name,
			Path:   name,
			Type:   config.RepoTypeSymlink,
			Source: target,
		}, nil
	}

	if !entry.IsDir() {
		return nil, nil
	}

	gitPath := filepath.Join(path, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil {
		return nil, nil
	}

	var repoType config.RepoType
	if gitInfo.IsDir() {
		repoType = config.RepoTypeClone
	} else {
		repoType = config.RepoTypeGit
	}

	source := gitRemoteURL(path)
	branch := gitCurrentBranch(path)

	return &config.Repository{
		Name:   name,
		Path:   name,
		Type:   repoType,
		Source: source,
		Branch: branch,
	}, nil
}

func gitRemoteURL(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitCurrentBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SyncOptions configures behavior of Sync.
type SyncOptions struct {
	Pull   bool // pull latest changes after switching branch
	Fetch  bool // fetch from remote before switching branch
	Prune  bool // prune deleted remote branches during fetch
	Submod bool // update submodules recursively
}

// SyncResult holds the outcome of a Sync operation.
type SyncResult struct {
	Synced  int
	Skipped int
	Failed  int
}

// Sync synchronizes repositories to match repos.yaml configuration.
// For each repository, it switches to the configured branch and optionally
// fetches/pulls latest changes.
func (w *Workspace) Sync(repoNames []string, opts SyncOptions) (*SyncResult, error) {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)
	result := &SyncResult{}

	for _, repo := range w.Config.Repositories {
		if len(repoNames) > 0 && !slices.Contains(repoNames, repo.Name) {
			continue
		}

		destPath := filepath.Join(wsDir, repo.Path)
		var err error
		var skipped bool
		switch {
		case repo.IsSymlink():
			skipped, err = w.syncSymlink(repo, destPath, opts)
		case repo.IsClone():
			skipped, err = w.syncClone(repo, destPath, opts)
		default:
			skipped, err = w.syncSubmodule(repo, destPath, opts)
		}
		if err != nil {
			output.PrintSyncFail(fmt.Sprintf("%v", err))
			result.Failed++
		} else if skipped {
			result.Skipped++
		} else {
			result.Synced++
		}
	}

	return result, nil
}

func (w *Workspace) syncSymlink(repo config.Repository, destPath string, opts SyncOptions) (bool, error) {
	output.PrintSyncHeader(repo.Name, "symlink")

	info, err := os.Lstat(destPath)
	if err != nil {
		output.PrintSyncSkip("missing (run 'xr init' to recreate)")
		return true, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("%s exists but is not a symlink", destPath)
	}

	// Resolve symlink target to operate on the actual directory
	realPath, err := filepath.EvalSymlinks(destPath)
	if err != nil {
		return false, fmt.Errorf("resolving symlink: %w", err)
	}

	// Check if the target is a git repository
	if _, err := os.Stat(filepath.Join(realPath, ".git")); os.IsNotExist(err) {
		output.PrintSyncSkip("not a git repository")
		return true, nil
	}

	// No branch configured — nothing to sync
	if repo.Branch == "" && !opts.Fetch && !opts.Pull {
		output.PrintSyncSkip("no branch configured")
		return true, nil
	}

	if err := w.syncGitRepo(repo, realPath, opts); err != nil {
		return false, err
	}

	return false, nil
}

func (w *Workspace) syncClone(repo config.Repository, destPath string, opts SyncOptions) (bool, error) {
	output.PrintSyncHeader(repo.Name, "clone")

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		output.PrintSyncSkip("missing (run 'xr init' to clone)")
		return true, nil
	}

	if err := w.syncGitRepo(repo, destPath, opts); err != nil {
		return false, err
	}

	// Update submodules within the clone
	if opts.Submod {
		if hasSubmodules(destPath) {
			output.PrintSyncAction("updating submodules")
			if err := runGitQuiet(destPath, "submodule", "update", "--init", "--recursive"); err != nil {
				return false, fmt.Errorf("submodule update: %w", err)
			}
			output.PrintSyncOK("submodules updated")
		}
	}

	return false, nil
}

func (w *Workspace) syncSubmodule(repo config.Repository, destPath string, opts SyncOptions) (bool, error) {
	output.PrintSyncHeader(repo.Name, "submodule")

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		output.PrintSyncSkip("missing (run 'xr init' to add)")
		return true, nil
	}

	// Initialize and update the submodule first
	relPath, err := filepath.Rel(w.Root, destPath)
	if err != nil {
		return false, fmt.Errorf("computing relative path: %w", err)
	}

	output.PrintSyncAction("initializing submodule")
	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive", "--", relPath)
	cmd.Dir = w.Root
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("submodule update: %s", strings.TrimSpace(string(out)))
	}

	if err := w.syncGitRepo(repo, destPath, opts); err != nil {
		return false, err
	}

	return false, nil
}

// syncGitRepo performs fetch, checkout, and pull on a git repository directory.
func (w *Workspace) syncGitRepo(repo config.Repository, dir string, opts SyncOptions) error {
	// Fetch from remote
	if opts.Fetch {
		args := []string{"fetch", "origin"}
		if opts.Prune {
			args = append(args, "--prune")
		}
		output.PrintSyncAction("fetching from origin")
		if err := runGitQuiet(dir, args...); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
	}

	// Switch to configured branch if specified
	if repo.Branch != "" {
		currentBranch := gitCurrentBranch(dir)
		if currentBranch == repo.Branch {
			output.PrintSyncOK(fmt.Sprintf("already on %s", repo.Branch))
		} else {
			output.PrintSyncAction(fmt.Sprintf("switching %s → %s", currentBranch, repo.Branch))
			if err := runGitQuiet(dir, "checkout", repo.Branch); err != nil {
				return fmt.Errorf("checkout %s: %w", repo.Branch, err)
			}
			output.PrintSyncOK(fmt.Sprintf("switched to %s", repo.Branch))
		}
	}

	// Pull latest changes
	if opts.Pull {
		branch := repo.Branch
		if branch == "" {
			branch = gitCurrentBranch(dir)
		}
		if branch == "" {
			output.PrintSyncFail("could not determine branch for pull")
		} else {
			output.PrintSyncAction(fmt.Sprintf("pulling origin/%s", branch))
			if err := runGitQuiet(dir, "pull", "origin", branch); err != nil {
				return fmt.Errorf("pull origin/%s: %w", branch, err)
			}
			output.PrintSyncOK("up to date")
		}
	}

	return nil
}

// runGitQuiet runs a git command, suppressing stdout/stderr. On error, returns
// the combined output trimmed as the error message.
func runGitQuiet(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

// hasSubmodules checks if a git repository has submodules.
func hasSubmodules(dir string) bool {
	gitmodulesPath := filepath.Join(dir, ".gitmodules")
	info, err := os.Stat(gitmodulesPath)
	return err == nil && info.Size() > 0
}
