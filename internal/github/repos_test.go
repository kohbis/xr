package github

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kohbis/xr/internal/config"
)

func TestListRepos(t *testing.T) {
	var gotArgs []string
	stubGH(t, func(args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`[
			{"name": "web", "url": "https://github.com/acme/web", "sshUrl": "git@github.com:acme/web.git", "defaultBranchRef": {"name": "trunk"}},
			{"name": "api", "url": "https://github.com/acme/api", "sshUrl": "git@github.com:acme/api.git", "defaultBranchRef": {"name": "main"}}
		]`), nil
	})

	repos, truncated, err := ListRepos("acme")
	if err != nil {
		t.Fatalf("ListRepos() error = %v", err)
	}
	want := []RemoteRepo{
		{Name: "api", URL: "https://github.com/acme/api", SSHURL: "git@github.com:acme/api.git", DefaultBranch: "main"},
		{Name: "web", URL: "https://github.com/acme/web", SSHURL: "git@github.com:acme/web.git", DefaultBranch: "trunk"},
	}
	if !reflect.DeepEqual(repos, want) || truncated {
		t.Errorf("ListRepos() = %+v, %v; want %+v (sorted), false", repos, truncated, want)
	}
	joined := strings.Join(gotArgs, " ")
	for _, part := range []string{"repo list acme", "--no-archived", "--source"} {
		if !strings.Contains(joined, part) {
			t.Errorf("gh args %q missing %q", joined, part)
		}
	}
}

func TestSelectByName(t *testing.T) {
	listed := []RemoteRepo{{Name: "api"}, {Name: "web"}, {Name: "docs"}}

	got, err := SelectByName(listed, []string{"docs", "api", "api"})
	if err != nil {
		t.Fatalf("SelectByName() error = %v", err)
	}
	if want := []RemoteRepo{{Name: "api"}, {Name: "docs"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("SelectByName() = %+v, want %+v (listed order, no duplicates)", got, want)
	}

	_, err = SelectByName(listed, []string{"api", "nope", "typo"})
	if err == nil || !strings.Contains(err.Error(), "nope, typo") {
		t.Errorf("SelectByName() with unknown names error = %v, want both named", err)
	}
}

func TestConfigRepository(t *testing.T) {
	r := RemoteRepo{Name: "api", URL: "https://github.com/acme/api", SSHURL: "git@github.com:acme/api.git", DefaultBranch: "main"}
	if got := ConfigRepository(r, ProtocolHTTPS).Source; got != "https://github.com/acme/api.git" {
		t.Errorf("https source = %q", got)
	}
	if got := ConfigRepository(r, "").Source; got != "git@github.com:acme/api.git" {
		t.Errorf("default source = %q, want ssh", got)
	}
	got := ConfigRepository(r, ProtocolSSH)
	want := config.Repository{Name: "api", Path: "api", Source: "git@github.com:acme/api.git", Branch: "main", Type: config.RepoTypeClone}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConfigRepository(ssh) = %+v, want %+v", got, want)
	}
}

func TestPlanImport(t *testing.T) {
	existing := []config.Repository{
		{Name: "api", Path: "api", Source: "https://github.com/acme/api.git"},  // same repo, other URL form
		{Name: "web", Path: "web", Source: "https://github.com/other/web.git"}, // name taken by another repo
	}
	listed := []RemoteRepo{
		{Name: "api", URL: "https://github.com/acme/api", SSHURL: "git@github.com:acme/api.git"},
		{Name: "web", URL: "https://github.com/acme/web", SSHURL: "git@github.com:acme/web.git"},
		{Name: "lib", URL: "https://github.com/acme/lib", SSHURL: "git@github.com:acme/lib.git"},
	}

	add, conflicts := PlanImport(existing, listed, "ssh")
	if len(add) != 1 || add[0].Name != "lib" {
		t.Errorf("add = %+v, want only lib", add)
	}
	if len(conflicts) != 1 || !strings.HasPrefix(conflicts[0], "web:") {
		t.Errorf("conflicts = %v, want one for web", conflicts)
	}
}

func TestSameRepository(t *testing.T) {
	if !SameRepository("git@github.com:Acme/API.git", "https://github.com/acme/api") {
		t.Error("ssh and https URLs of one repository must match")
	}
	if SameRepository("https://github.com/acme/api", "https://ghe.example.com/acme/api") {
		t.Error("the same owner/name on another host must not match")
	}
	if SameRepository("/local/api", "/local/api") {
		t.Error("non-GitHub sources are not compared")
	}
}
