package workspace

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
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
			output.PrintStep(fmt.Sprintf("%s is already in .gitignore", entry))
			return nil
		}
		content := string(existing)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry + "\n"
		output.PrintStep(fmt.Sprintf("adding %s to .gitignore", entry))
		return os.WriteFile(gitignorePath, []byte(content), 0644)
	}

	output.PrintStep(".gitignore unchanged")
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
	output.PrintStep("creating README.md")
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func (w *Workspace) addRepo(repo config.Repository, wsDir string) error {
	destPath := filepath.Join(wsDir, repo.Path)
	if repo.IsSymlink() {
		return w.addSymlink(repo, destPath)
	}
	return w.addClone(repo, destPath)
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
		output.PrintStep(fmt.Sprintf("symlink %s already exists, skipping", repo.Name))
		return nil
	}
	source := expandTilde(repo.Source)
	output.PrintStep(fmt.Sprintf("creating symlink %s -> %s", repo.Name, source))
	return createSymlink(repo, destPath)
}

// createSymlink links destPath to the repository source. It assumes destPath
// does not exist yet and emits no progress output of its own.
func createSymlink(repo config.Repository, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	if err := os.Symlink(expandTilde(repo.Source), destPath); err != nil {
		return fmt.Errorf("creating symlink: %w", err)
	}
	return nil
}

func (w *Workspace) addClone(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); err == nil {
		output.PrintStep(fmt.Sprintf("clone %s already exists, skipping", repo.Name))
		return nil
	}

	output.PrintStep(fmt.Sprintf("cloning %s from %s", repo.Name, repo.Source))
	return w.cloneRepo(repo, destPath, os.Stdout, os.Stderr)
}

// cloneRepo clones the repository into destPath, streaming git's own progress
// to stdout and stderr. It assumes destPath does not exist yet and emits no
// progress output of its own.
func (w *Workspace) cloneRepo(repo config.Repository, destPath string, stdout, stderr io.Writer) error {
	if repo.Source == "" {
		return fmt.Errorf("no source configured")
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	args := []string{"clone"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.Source, destPath)

	if err := git.RunWithIO(w.Root, stdout, stderr, args...); err != nil {
		return fmt.Errorf("git clone: %w", err)
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
		if repo.IsSymlink() {
			err = w.removeSymlink(repo, destPath)
		} else {
			err = w.removeClone(repo, destPath)
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
		output.PrintStep(fmt.Sprintf("symlink %s already removed", repo.Name))
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists but is not a symlink", destPath)
	}
	output.PrintStep(fmt.Sprintf("removing symlink %s", repo.Name))
	return os.Remove(destPath)
}

func (w *Workspace) removeClone(repo config.Repository, destPath string) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		output.PrintStep(fmt.Sprintf("clone %s already removed", repo.Name))
		return nil
	}
	output.PrintStep(fmt.Sprintf("removing clone %s", repo.Name))
	return os.RemoveAll(destPath)
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
			output.PrintStepWarning(fmt.Sprintf("skipping %s: %v", entry.Name(), err))
			continue
		}
		if repo == nil {
			continue
		}
		if repo.Source == "" {
			output.PrintStepWarning(fmt.Sprintf("%s: no origin remote found, source will be empty", repo.Name))
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
	if _, err := os.Stat(gitPath); err != nil {
		return nil, nil
	}

	source, _ := git.RemoteURL(path)
	branch := gitCurrentBranch(path)

	return &config.Repository{
		Name:   name,
		Path:   name,
		Type:   config.RepoTypeClone,
		Source: source,
		Branch: branch,
	}, nil
}

func gitCurrentBranch(dir string) string {
	branch, err := git.CurrentBranch(dir)
	if err != nil {
		return ""
	}
	return branch
}

// SyncOptions configures behavior of Sync.
type SyncOptions struct {
	Pull   bool // pull latest changes after switching branch
	Fetch  bool // fetch from remote before switching branch
	Prune  bool // prune deleted remote branches during fetch
	DryRun bool // show what would be done, perform no actions

	// CloneMissing materializes repositories that are absent from the workspace
	// directory: clone repos are cloned, symlink repos are linked. Without it a
	// missing repository is skipped. This is what makes an unattended bootstrap
	// from repos.yaml possible, since xr init is interactive only.
	CloneMissing bool

	// CreateBranchIfMissing creates a local branch when checkout fails and the
	// remote tracking branch is also unavailable. The branch is created from the
	// current HEAD.
	CreateBranchIfMissing bool

	// AllowDirty controls behavior when the working tree is dirty and sync would
	// change branches or pull. When false, the repo is skipped unless ConfirmDirty
	// returns true.
	AllowDirty bool

	// ConfirmDirty is an optional callback used to decide whether to proceed when
	// the working tree is dirty and sync would be disruptive (checkout/pull).
	// Return true to proceed, false to skip.
	ConfirmDirty func(repo config.Repository, reason string) (bool, error)

	// ConfirmCheckout is an optional callback used to confirm switching branches.
	// Return true to proceed with checkout, false to skip the repo.
	ConfirmCheckout func(repo config.Repository, fromBranch, toBranch string) (bool, error)

	// Jobs is the number of repositories synced concurrently. Values below 2 sync
	// sequentially, streaming each repository's output as it happens. Above that,
	// output is buffered per repository and flushed in configuration order, so the
	// result reads the same but appears in bursts.
	//
	// Sync falls back to sequential whenever ConfirmDirty or ConfirmCheckout is
	// set, since prompts read from stdin and must not interleave.
	Jobs int
}

// SyncResult holds the outcome of a Sync operation.
type SyncResult struct {
	Synced  int
	Skipped int
	Failed  int
}

// record counts a repository outcome.
func (r *SyncResult) record(skipped bool, err error) {
	switch {
	case err != nil:
		r.Failed++
	case skipped:
		r.Skipped++
	default:
		r.Synced++
	}
}

// Sync synchronizes repositories to match repos.yaml configuration.
// For each repository, it switches to the configured branch and optionally
// fetches/pulls latest changes. Repositories are synced concurrently when
// opts.Jobs allows it; see SyncOptions.Jobs.
func (w *Workspace) Sync(repoNames []string, opts SyncOptions) (*SyncResult, error) {
	wsDir := filepath.Join(w.Root, w.Config.Workspace)

	targets := make([]config.Repository, 0, len(w.Config.Repositories))
	for _, repo := range w.Config.Repositories {
		if len(repoNames) > 0 && !slices.Contains(repoNames, repo.Name) {
			continue
		}
		targets = append(targets, repo)
	}

	if w.syncJobs(opts, len(targets)) > 1 {
		return w.syncConcurrent(targets, wsDir, opts), nil
	}

	result := &SyncResult{}
	p := output.StdoutSyncPrinter()
	for _, repo := range targets {
		skipped, err := w.syncRepo(repo, wsDir, opts, p)
		result.record(skipped, err)
	}
	return result, nil
}

// syncJobs resolves the effective worker count for n repositories.
func (w *Workspace) syncJobs(opts SyncOptions, n int) int {
	// Prompts read from stdin, so they cannot run from concurrent workers.
	if opts.ConfirmDirty != nil || opts.ConfirmCheckout != nil {
		return 1
	}
	jobs := opts.Jobs
	if jobs < 1 {
		jobs = 1
	}
	if jobs > n {
		jobs = n
	}
	return jobs
}

// syncRepo syncs one repository, reporting failures through p. It returns
// whether the repository was skipped along with any error, both already
// reflected in the printed output.
func (w *Workspace) syncRepo(repo config.Repository, wsDir string, opts SyncOptions, p *output.SyncPrinter) (bool, error) {
	destPath := filepath.Join(wsDir, repo.Path)

	var skipped bool
	var err error
	if repo.IsSymlink() {
		skipped, err = w.syncSymlink(repo, destPath, opts, p)
	} else {
		skipped, err = w.syncClone(repo, destPath, opts, p)
	}
	if err != nil {
		p.Fail(fmt.Sprintf("%v", err))
	}
	return skipped, err
}

// syncConcurrent syncs repositories using jobs workers. Each repository renders
// into its own buffer, and buffers are flushed in configuration order so the
// output is identical to a sequential run.
func (w *Workspace) syncConcurrent(targets []config.Repository, wsDir string, opts SyncOptions) *SyncResult {
	type slot struct {
		buf     bytes.Buffer
		skipped bool
		err     error
		done    chan struct{}
	}

	slots := make([]*slot, len(targets))
	for i := range slots {
		slots[i] = &slot{done: make(chan struct{})}
	}

	queue := make(chan int)
	go func() {
		for i := range targets {
			queue <- i
		}
		close(queue)
	}()

	var wg sync.WaitGroup
	for range w.syncJobs(opts, len(targets)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				s := slots[i]
				s.skipped, s.err = w.syncRepo(targets[i], wsDir, opts, output.NewSyncPrinter(&s.buf))
				close(s.done)
			}
		}()
	}

	result := &SyncResult{}
	for _, s := range slots {
		<-s.done
		_, _ = io.Copy(os.Stdout, &s.buf)
		result.record(s.skipped, s.err)
	}
	wg.Wait()

	return result
}

func (w *Workspace) syncSymlink(repo config.Repository, destPath string, opts SyncOptions, p *output.SyncPrinter) (bool, error) {
	p.Header(repo.Name, "symlink")

	info, err := os.Lstat(destPath)
	switch {
	case err != nil && !opts.CloneMissing:
		p.Skip("missing (use --clone-missing or run 'xr init')")
		return true, nil
	case err != nil && opts.DryRun:
		p.Action(fmt.Sprintf("preview: would link to %s", expandTilde(repo.Source)))
		p.Skip("preview")
		return true, nil
	case err != nil:
		p.Action(fmt.Sprintf("linking to %s", expandTilde(repo.Source)))
		if err := createSymlink(repo, destPath); err != nil {
			return false, err
		}
		p.OK("symlink created")
	default:
		if info.Mode()&os.ModeSymlink == 0 {
			return false, fmt.Errorf("%s exists but is not a symlink", destPath)
		}
	}

	// Resolve symlink target to operate on the actual directory
	realPath, err := filepath.EvalSymlinks(destPath)
	if err != nil {
		return false, fmt.Errorf("resolving symlink: %w", err)
	}

	// Check if the target is a git repository
	if _, err := os.Stat(filepath.Join(realPath, ".git")); os.IsNotExist(err) {
		p.Skip("not a git repository")
		return true, nil
	}

	// No branch configured — nothing to sync
	if repo.Branch == "" && !opts.Fetch && !opts.Pull {
		p.Skip("no branch configured")
		return true, nil
	}

	skipped, err := w.syncGitRepo(repo, realPath, opts, p)
	if err != nil {
		return false, err
	}

	return skipped, nil
}

func (w *Workspace) syncClone(repo config.Repository, destPath string, opts SyncOptions, p *output.SyncPrinter) (bool, error) {
	p.Header(repo.Name, "clone")

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if !opts.CloneMissing {
			p.Skip("missing (use --clone-missing or run 'xr init')")
			return true, nil
		}
		if opts.DryRun {
			p.Action(fmt.Sprintf("preview: would clone %s", repo.Source))
			p.Skip("preview")
			return true, nil
		}
		p.Action(fmt.Sprintf("cloning from %s", repo.Source))
		// git's clone progress joins the repository's own output, so it stays with
		// this repository's block when workers run concurrently.
		if err := w.cloneRepo(repo, destPath, p.Writer(), p.Writer()); err != nil {
			return false, err
		}
		// A fresh clone is already on the configured branch and up to date, so
		// there is nothing left for syncGitRepo to do.
		p.OK("cloned")
		return false, nil
	}

	skipped, err := w.syncGitRepo(repo, destPath, opts, p)
	if err != nil {
		return false, err
	}

	return skipped, nil
}

// syncGitRepo performs fetch, checkout, and pull on a git repository directory.
func (w *Workspace) syncGitRepo(repo config.Repository, dir string, opts SyncOptions, p *output.SyncPrinter) (bool, error) {
	currentBranch := gitCurrentBranch(dir)
	dirty, err := gitIsDirty(dir)
	if err != nil {
		return false, err
	}

	needsCheckout := repo.Branch != "" && currentBranch != repo.Branch
	needsPull := opts.Pull

	if needsCheckout && opts.ConfirmCheckout != nil && !opts.DryRun {
		ok, err := opts.ConfirmCheckout(repo, currentBranch, repo.Branch)
		if err != nil {
			return false, err
		}
		if !ok {
			p.Skip("checkout skipped")
			return true, nil
		}
	}

	if dirty && (needsCheckout || needsPull) && !opts.AllowDirty {
		reason := "dirty working tree"
		if needsCheckout && needsPull {
			reason += " (would checkout and pull)"
		} else if needsCheckout {
			reason += " (would checkout)"
		} else {
			reason += " (would pull)"
		}

		if opts.ConfirmDirty != nil {
			ok, err := opts.ConfirmDirty(repo, reason)
			if err != nil {
				return false, err
			}
			if !ok {
				p.Skip("dirty; skipped")
				return true, nil
			}
		} else {
			p.Skip("dirty; skipped (use --allow-dirty)")
			return true, nil
		}
	}

	if opts.DryRun {
		if opts.Fetch {
			if opts.Prune {
				p.Action("preview: would fetch origin --prune")
			} else {
				p.Action("preview: would fetch origin")
			}
		}
		if needsCheckout {
			p.Action(fmt.Sprintf("preview: would checkout %s", repo.Branch))
		}
		if opts.Pull {
			branch := repo.Branch
			if branch == "" {
				branch = currentBranch
			}
			if branch == "" {
				p.Fail("preview: could not determine branch for pull")
			} else {
				p.Action(fmt.Sprintf("preview: would pull origin/%s", branch))
			}
		}
		p.Skip("preview")
		return true, nil
	}

	// Fetch from remote
	if opts.Fetch {
		args := []string{"fetch", "origin"}
		if opts.Prune {
			args = append(args, "--prune")
		}
		p.Action("fetching from origin")
		if err := runGitQuiet(dir, args...); err != nil {
			return false, fmt.Errorf("fetch: %w", err)
		}
	}

	// Switch to configured branch if specified
	if repo.Branch != "" {
		if opts.CreateBranchIfMissing && !opts.Fetch {
			return false, fmt.Errorf("create-branch-if-missing requires fetch")
		}
		if currentBranch == repo.Branch {
			p.OK(fmt.Sprintf("already on %s", repo.Branch))
		} else {
			p.Action(fmt.Sprintf("switching %s → %s", currentBranch, repo.Branch))
			if err := runGitQuiet(dir, "checkout", repo.Branch); err != nil {
				remoteExists, rerr := gitRefExists(dir, "refs/remotes/origin/"+repo.Branch)
				if rerr != nil {
					return false, fmt.Errorf("checkout %s: %w", repo.Branch, err)
				}
				if remoteExists {
					if err2 := runGitQuiet(dir, "checkout", "-b", repo.Branch, "--track", "origin/"+repo.Branch); err2 != nil {
						return false, fmt.Errorf("checkout %s: %w", repo.Branch, err)
					}
				} else if opts.CreateBranchIfMissing {
					localExists, lerr := gitRefExists(dir, "refs/heads/"+repo.Branch)
					if lerr != nil {
						return false, fmt.Errorf("checkout %s: %w", repo.Branch, err)
					}
					if localExists {
						return false, fmt.Errorf("checkout %s: %w", repo.Branch, err)
					}
					if err3 := runGitQuiet(dir, "checkout", "-b", repo.Branch); err3 != nil {
						return false, fmt.Errorf("create branch %s: %w", repo.Branch, err3)
					}
				} else {
					return false, fmt.Errorf("checkout %s: %w", repo.Branch, err)
				}
			}
			p.OK(fmt.Sprintf("switched to %s", repo.Branch))
		}
	}

	// Pull latest changes
	if opts.Pull {
		branch := repo.Branch
		if branch == "" {
			branch = gitCurrentBranch(dir)
		}
		if branch == "" {
			p.Fail("could not determine branch for pull")
		} else {
			p.Action(fmt.Sprintf("pulling origin/%s", branch))
			if err := runGitQuiet(dir, "pull", "origin", branch); err != nil {
				return false, fmt.Errorf("pull origin/%s: %w", branch, err)
			}
			p.OK("up to date")
		}
	}

	return false, nil
}

func gitIsDirty(dir string) (bool, error) {
	return git.IsDirty(dir)
}

// runGitQuiet runs a git command, suppressing stdout/stderr. On error, returns
// the combined output trimmed as the error message.
func runGitQuiet(dir string, args ...string) error {
	return git.RunQuiet(dir, args...)
}

func gitRefExists(dir, ref string) (bool, error) {
	err := git.Run(dir, "rev-parse", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	// rev-parse returns exit code 1 when ref is missing.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
