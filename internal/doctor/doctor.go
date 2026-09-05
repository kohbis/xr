// Package doctor reports whether the environment xr needs is in place: the
// external tools it shells out to, and the workspace the commands operate on.
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
}

// Report is the outcome of every check, in the order they were run.
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
		add(workspaceCheck(cfg))
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
func workspaceCheck(cfg *config.Config) Check {
	wsDir, err := cfg.WorkspaceDir()
	if err != nil {
		return Check{
			Name:   "workspace",
			Status: StatusFailed,
			Detail: err.Error(),
		}
	}

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

// joinLimit lists at most n names, so a wholly unmaterialized workspace does
// not print one line per repository.
func joinLimit(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:n], ", "), len(names)-n)
}
