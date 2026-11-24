// Package github reads what GitHub knows about a repository and git does not,
// such as pull request state, by running the gh CLI.
//
// gh is an optional dependency: only the features that read GitHub use this
// package, each behind an explicit flag, and they fail with ErrNotInstalled
// when gh is missing. Setting XR_NO_GH turns gh off even where it is installed:
// those features then fail with ErrDisabled and xr never runs gh. Like
// internal/git for the git binary, this is the only package that runs gh.
package github

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/kohbis/xr/internal/git"
)

// ErrNotInstalled is returned when gh is not on PATH.
var ErrNotInstalled = errors.New("gh is not installed: this feature reads GitHub through the gh CLI (see 'xr doctor')")

// DisableEnv is the environment variable that turns gh off. Any value other
// than empty, "0" or "false" disables it.
const DisableEnv = "XR_NO_GH"

// ErrDisabled is returned when gh has been turned off with DisableEnv.
var ErrDisabled = errors.New("gh is disabled by " + DisableEnv + ": this feature reads GitHub through the gh CLI")

// ErrNotGitHub is returned for a repository whose origin is not on a GitHub
// host gh can reach.
var ErrNotGitHub = errors.New("origin is not a GitHub repository")

// lookPath and run are variables so tests can stand in for gh.
var (
	lookPath = exec.LookPath
	run      = runGH
)

func runGH(args ...string) ([]byte, error) {
	if Disabled() {
		return nil, ErrDisabled
	}
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("gh %s: %w", args[0], err)
	}
	return out, nil
}

// Disabled reports whether gh has been turned off with DisableEnv.
func Disabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DisableEnv))) {
	case "", "0", "false":
		return false
	}
	return true
}

// Available reports why gh cannot be used — ErrDisabled or ErrNotInstalled —
// or nil when it can. Features call it before anything else, so a disabled gh
// is never run.
func Available() error {
	if Disabled() {
		return ErrDisabled
	}
	if _, err := lookPath("gh"); err != nil {
		return ErrNotInstalled
	}
	return nil
}

// Repo identifies a repository on a GitHub host.
type Repo struct {
	Host  string
	Owner string
	Name  string
}

// String is the HOST/OWNER/NAME form gh accepts for --repo.
func (r Repo) String() string {
	return r.Host + "/" + r.Owner + "/" + r.Name
}

// ParseRemote extracts host, owner and name from a git remote URL: https and
// ssh URLs (https://host/owner/name, ssh://git@host:22/owner/name) and the
// scp-like form (git@host:owner/name), with or without a trailing .git. It does
// not decide whether the host is a GitHub host.
func ParseRemote(remote string) (Repo, bool) {
	remote = strings.TrimSpace(remote)
	var host, path string

	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil {
			return Repo{}, false
		}
		host, path = u.Hostname(), u.Path
	} else {
		// scp-like: [user@]host:owner/name
		at := strings.LastIndex(remote, "@")
		rest := remote[at+1:]
		h, p, ok := strings.Cut(rest, ":")
		if !ok {
			return Repo{}, false
		}
		host, path = h, p
	}

	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	owner, name, ok := strings.Cut(path, "/")
	if !ok || host == "" || owner == "" || name == "" || strings.Contains(name, "/") {
		return Repo{}, false
	}
	return Repo{Host: strings.ToLower(host), Owner: owner, Name: name}, true
}

var (
	hostMu    sync.Mutex
	hostCache = map[string]bool{}
)

// isGitHubHost reports whether gh can talk to host: github.com always, any
// other host (GitHub Enterprise) when gh has credentials for it.
func isGitHubHost(host string) bool {
	if host == "github.com" {
		return true
	}
	hostMu.Lock()
	defer hostMu.Unlock()
	if known, ok := hostCache[host]; ok {
		return known
	}
	_, err := run("auth", "token", "--hostname", host)
	hostCache[host] = err == nil
	return err == nil
}

// RepoFromOrigin returns the GitHub repository the origin remote of the git
// repository at repoDir points at, or ErrNotGitHub.
func RepoFromOrigin(repoDir string) (Repo, error) {
	remote, err := git.RemoteURL(repoDir)
	if err != nil {
		return Repo{}, fmt.Errorf("reading origin: %w", err)
	}
	repo, ok := ParseRemote(remote)
	if !ok || !isGitHubHost(repo.Host) {
		return Repo{}, fmt.Errorf("%w: %s", ErrNotGitHub, remote)
	}
	return repo, nil
}
