package github

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kohbis/xr/internal/config"
)

// listLimit caps how many repositories ListRepos asks gh for.
const listLimit = 1000

// RemoteRepo is a repository as GitHub lists it.
type RemoteRepo struct {
	Name          string
	URL           string // https, without .git
	SSHURL        string
	DefaultBranch string
}

type ghRepo struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	SSHURL           string `json:"sshUrl"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

// ListRepos returns the repositories owned by owner (a user or organization)
// on the host gh targets (github.com, or GH_HOST), sorted by name. Archived
// repositories and forks are left out. truncated reports that the listing
// stopped at its limit.
func ListRepos(owner string) (repos []RemoteRepo, truncated bool, err error) {
	out, err := run("repo", "list", owner,
		"--limit", strconv.Itoa(listLimit),
		"--no-archived", "--source",
		"--json", "name,url,sshUrl,defaultBranchRef")
	if err != nil {
		return nil, false, err
	}
	var listed []ghRepo
	if err := json.Unmarshal(out, &listed); err != nil {
		return nil, false, fmt.Errorf("parsing gh repo list output: %w", err)
	}
	repos = make([]RemoteRepo, 0, len(listed))
	for _, r := range listed {
		repos = append(repos, RemoteRepo{
			Name:          r.Name,
			URL:           r.URL,
			SSHURL:        r.SSHURL,
			DefaultBranch: r.DefaultBranchRef.Name,
		})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, len(listed) >= listLimit, nil
}

// SelectByName returns the repositories named in names, in listed order. A
// name that is not listed is an error, so a typo does not silently import
// nothing.
func SelectByName(listed []RemoteRepo, names []string) ([]RemoteRepo, error) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var selected []RemoteRepo
	for _, r := range listed {
		if want[r.Name] {
			selected = append(selected, r)
			delete(want, r.Name)
		}
	}
	if len(want) > 0 {
		var missing []string
		for _, n := range names {
			if want[n] {
				missing = append(missing, n)
				delete(want, n)
			}
		}
		return nil, fmt.Errorf("no such repository (archived repositories and forks are not listed): %s", strings.Join(missing, ", "))
	}
	return selected, nil
}

// Clone protocols for ConfigRepository.
const (
	ProtocolSSH   = "ssh"
	ProtocolHTTPS = "https"
)

// ConfigRepository turns a listed repository into a repos.yaml entry: a clone
// of its URL in protocol (ssh unless ProtocolHTTPS), checked out on its
// default branch.
func ConfigRepository(r RemoteRepo, protocol string) config.Repository {
	source := r.SSHURL
	if protocol == ProtocolHTTPS || source == "" {
		source = r.URL + ".git"
	}
	return config.Repository{
		Name:   r.Name,
		Path:   r.Name,
		Source: source,
		Branch: r.DefaultBranch,
		Type:   config.RepoTypeClone,
	}
}

// SameRepository reports whether two sources point at the same GitHub
// repository, whichever URL form each uses.
func SameRepository(a, b string) bool {
	ra, okA := ParseRemote(a)
	rb, okB := ParseRemote(b)
	return okA && okB &&
		ra.Host == rb.Host &&
		strings.EqualFold(ra.Owner, rb.Owner) &&
		strings.EqualFold(ra.Name, rb.Name)
}

// PlanImport picks the listed repositories that can be added to a config that
// already has existing: a repository already there under any URL form is left
// out silently, and one whose name or path is taken by a different repository
// is left out and reported in conflicts, so nothing in repos.yaml is
// overwritten.
func PlanImport(existing []config.Repository, listed []RemoteRepo, protocol string) (add []config.Repository, conflicts []string) {
	names := map[string]bool{}
	paths := map[string]bool{}
	for _, e := range existing {
		names[e.Name] = true
		paths[e.Path] = true
	}

listed:
	for _, r := range listed {
		repo := ConfigRepository(r, protocol)
		for _, e := range existing {
			if SameRepository(e.Source, repo.Source) {
				continue listed
			}
		}
		if names[repo.Name] || paths[repo.Path] {
			conflicts = append(conflicts, fmt.Sprintf("%s: name or path already used by another repository in repos.yaml", repo.Name))
			continue
		}
		names[repo.Name] = true
		paths[repo.Path] = true
		add = append(add, repo)
	}
	return add, conflicts
}
