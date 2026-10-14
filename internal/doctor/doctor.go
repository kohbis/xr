// Package doctor reports whether the environment xr needs is in place: the
// external tools it shells out to, the workspace the commands operate on, and
// whether the repositories in it still match what repos.yaml declares.
//
// It answers "why does xr not work here" before a command fails halfway
// through, which matters most in CI, where the failure would otherwise surface
// as a subprocess error from inside an unrelated command.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
)

// Status is the outcome of a single check.
const (
	// StatusOK means the check found what it needed.
	StatusOK = "ok"
	// StatusWarning means xr still works, but in a reduced or not-yet-set-up
	// form: ripgrep absent, or a workspace that has not been materialized.
	StatusWarning = "warning"
	// StatusFailed means a command will fail: a required tool is missing, or a
	// config file exists but cannot be used.
	StatusFailed = "failed"
)

// Check is one diagnosed aspect of the environment.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Hint is what to do about a warning or a failure.
	Hint string `json:"hint,omitempty"`
	// Issues breaks a check down per repository, for the checks that look at
	// each one. A check with issues has Status warning.
	Issues []Issue `json:"issues,omitempty"`
}

// Issue is one way a repository on disk differs from its entry in repos.yaml.
type Issue struct {
	Repo   string `json:"repo"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

// Report is the outcome of every check, in the order they were run.
//
// Warnings are counted apart from Failed on purpose: a workspace that has not
// been cloned yet, or a missing optional tool, describes a setup that is not
// finished rather than one that is broken. Only Failed gates the exit status.
type Report struct {
	Checks   []Check `json:"checks"`
	OK       int     `json:"ok"`
	Warnings int     `json:"warnings"`
	Failed   int     `json:"failed"`
}

// lookPath resolves an external tool. It is a variable so tests can simulate a
// machine where a tool is missing.
var lookPath = exec.LookPath

// Run diagnoses the environment for the config at cfgPath, which is the path a
// command would use (see config.CommandPath).
func Run(cfgPath string) Report {
	var report Report

	add := func(c Check) {
		report.Checks = append(report.Checks, c)
		switch c.Status {
		case StatusFailed:
			report.Failed++
		case StatusWarning:
			report.Warnings++
		default:
			report.OK++
		}
	}

	add(requiredTool("git", "xr init, xr repo sync, xr repo import, xr worktree, xr diff"))
	add(requiredTool("diff", "xr diff file"))
	add(optionalTool("rg", "xr search", "xr search falls back to its built-in engine, with the same results"))

	cfg, cfgCheck := configCheck(cfgPath)
	add(cfgCheck)
	if cfg != nil {
		wsDir, err := cfg.WorkspaceDir()
		if err != nil {
			add(Check{Name: "workspace", Status: StatusFailed, Detail: err.Error()})
		} else {
			add(workspaceCheck(cfg, wsDir))
			add(repositoriesCheck(cfg, wsDir))
		}
	}

	return report
}

func requiredTool(name, usedBy string) Check {
	path, err := lookPath(name)
	if err != nil {
		return Check{
			Name:   name,
			Status: StatusFailed,
			Detail: "not found in PATH",
			Hint:   "required by " + usedBy,
		}
	}
	return Check{Name: name, Status: StatusOK, Detail: path}
}

func optionalTool(name, usedBy, fallback string) Check {
	path, err := lookPath(name)
	if err != nil {
		return Check{
			Name:   name,
			Status: StatusWarning,
			Detail: "not found in PATH (optional, used by " + usedBy + ")",
			Hint:   fallback,
		}
	}
	return Check{Name: name, Status: StatusOK, Detail: path}
}

// configCheck reports the config a command would load. A missing config is a
// warning rather than a failure: it is the expected state before a workspace
// exists. A config that is present but unusable is a failure.
func configCheck(cfgPath string) (*config.Config, Check) {
	if _, err := os.Stat(cfgPath); err != nil {
		return nil, Check{
			Name:   "config",
			Status: StatusWarning,
			Detail: "no repos.yaml found in this or any parent directory",
			Hint:   "run 'xr init' here, or pass --config",
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, Check{
			Name:   "config",
			Status: StatusFailed,
			Detail: err.Error(),
			Hint:   "fix " + cfgPath,
		}
	}
	return cfg, Check{
		Name:   "config",
		Status: StatusOK,
		Detail: fmt.Sprintf("%s (%d repositories)", cfg.Path, len(cfg.Repositories)),
	}
}

// workspaceCheck reports how much of the workspace is materialized. Missing
// repositories are a warning, matching the commands, which skip them rather
// than failing.
func workspaceCheck(cfg *config.Config, wsDir string) Check {
	var missing []string
	for _, repo := range cfg.Repositories {
		if _, err := os.Lstat(filepath.Join(wsDir, repo.Path)); err != nil {
			missing = append(missing, repo.Name)
		}
	}
	if len(missing) > 0 {
		return Check{
			Name:   "workspace",
			Status: StatusWarning,
			Detail: fmt.Sprintf("%s (%d of %d repositories missing: %s)", wsDir, len(missing), len(cfg.Repositories), joinLimit(missing, 5)),
			Hint:   "run 'xr repo sync --clone-missing'",
		}
	}
	return Check{
		Name:   "workspace",
		Status: StatusOK,
		Detail: fmt.Sprintf("%s (%d repositories present)", wsDir, len(cfg.Repositories)),
	}
}

// repositoriesCheck reports repositories whose state on disk has drifted from
// their repos.yaml entry: a symlink that points elsewhere or at nothing, a
// clone whose origin is another URL, a directory that is not a git repository,
// or a checkout on a different branch than the one configured.
//
// Every finding is a warning. The commands keep working on such a repository
// (sync skips what it cannot handle and list marks it), so the run reports
// drift without failing; what fails the exit status stays limited to the tools
// and the config. Missing repositories are left to workspaceCheck.
func repositoriesCheck(cfg *config.Config, wsDir string) Check {
	var issues []Issue
	present := 0
	drifted := 0
	for _, repo := range cfg.Repositories {
		path := filepath.Join(wsDir, repo.Path)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		present++
		found := inspectRepo(repo, path, info)
		if len(found) > 0 {
			drifted++
		}
		issues = append(issues, found...)
	}

	if present == 0 {
		return Check{Name: "repositories", Status: StatusOK, Detail: "none present to check"}
	}
	if drifted > 0 {
		return Check{
			Name:   "repositories",
			Status: StatusWarning,
			Detail: fmt.Sprintf("%d of %d present differ from repos.yaml", drifted, present),
			Issues: issues,
		}
	}
	return Check{
		Name:   "repositories",
		Status: StatusOK,
		Detail: fmt.Sprintf("%d present match repos.yaml", present),
	}
}

// inspectRepo lists how one present repository differs from its entry. The
// shape on disk (symlink or directory) is checked first; git is consulted only
// when there is a repository to ask.
func inspectRepo(repo config.Repository, path string, info os.FileInfo) []Issue {
	var issues []Issue
	issue := func(detail, hint string) {
		issues = append(issues, Issue{Repo: repo.Name, Detail: detail, Hint: hint})
	}

	isLink := info.Mode()&os.ModeSymlink != 0
	switch {
	case repo.IsSymlink() && !isLink:
		issue("is a directory, but type is symlink", "remove it and run 'xr repo sync --clone-missing' to link "+repo.Source)
	case repo.IsSymlink():
		target, err := os.Readlink(path)
		if err != nil {
			issue("cannot read symlink: "+err.Error(), "")
			return issues
		}
		if !sameSymlinkTarget(path, target, expandTilde(repo.Source)) {
			issue(fmt.Sprintf("links to %s, source is %s", target, repo.Source), "fix the link, or update source in repos.yaml")
		}
		if _, err := os.Stat(path); err != nil {
			issue("symlink target does not exist", "restore "+target+", or update source in repos.yaml")
			return issues
		}
	case isLink:
		target, _ := os.Readlink(path)
		issue(fmt.Sprintf("is a symlink to %s, but type is clone", target), "set type: symlink in repos.yaml, or replace the link with a clone")
	}

	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		issue("not a git repository", "remove it and run 'xr repo sync --clone-missing'")
		return issues
	}

	if repo.Branch != "" {
		current, err := git.CurrentBranch(path)
		switch {
		case err != nil:
			issue("cannot read current branch: "+err.Error(), "")
		case current == "(detached)":
			issue("detached HEAD, repos.yaml says "+repo.Branch, "run 'xr repo sync'")
		case current != repo.Branch:
			issue(fmt.Sprintf("on branch %s, repos.yaml says %s", current, repo.Branch), "run 'xr repo sync'")
		}
	}

	if repo.IsClone() {
		url, err := git.RemoteURL(path)
		switch {
		case err != nil:
			issue("no origin remote", "git remote add origin "+repo.Source)
		case !sameRemote(url, repo.Source):
			issue(fmt.Sprintf("origin is %s, source is %s", url, repo.Source), "update source in repos.yaml, or 'git remote set-url origin "+repo.Source+"'")
		}
	}

	return issues
}

// sameSymlinkTarget compares the recorded link target with the source, first
// textually and then by what the two resolve to, so a relative link or a path
// through another symlink does not count as drift.
func sameSymlinkTarget(linkPath, target, source string) bool {
	if filepath.Clean(target) == filepath.Clean(source) {
		return true
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	resolvedTarget, err1 := filepath.EvalSymlinks(target)
	resolvedSource, err2 := filepath.EvalSymlinks(source)
	return err1 == nil && err2 == nil && resolvedTarget == resolvedSource
}

// sameRemote compares two git URLs loosely enough that a trailing slash or a
// ".git" suffix does not count as drift. It does not equate different
// protocols (ssh and https) for the same repository: which one is used is the
// user's choice, and the recorded source is what a fresh clone would get.
func sameRemote(a, b string) bool {
	return normalizeRemote(a) == normalizeRemote(b)
}

func normalizeRemote(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")
	return url
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

// joinLimit lists at most n names, so a wholly unmaterialized workspace does
// not print one line per repository.
func joinLimit(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:n], ", "), len(names)-n)
}
