package github

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Pull request states, lower-cased from GitHub's.
const (
	StateOpen   = "open"
	StateClosed = "closed" // closed without merging
	StateMerged = "merged"
)

// Check summaries.
const (
	ChecksPassing = "passing"
	ChecksFailing = "failing"
	ChecksPending = "pending"
)

// PullRequest is what xr reports about the pull request of a branch.
type PullRequest struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Draft  bool   `json:"draft,omitempty"`
	URL    string `json:"url"`
	// Head is the commit the pull request's branch pointed at on GitHub.
	Head string `json:"head"`
	// Checks summarizes the status checks on Head; empty when there are none.
	Checks string `json:"checks,omitempty"`
}

type ghPullRequest struct {
	Number            int        `json:"number"`
	State             string     `json:"state"`
	IsDraft           bool       `json:"isDraft"`
	URL               string     `json:"url"`
	HeadRefOid        string     `json:"headRefOid"`
	IsCrossRepository bool       `json:"isCrossRepository"`
	CreatedAt         time.Time  `json:"createdAt"`
	StatusCheckRollup []ghStatus `json:"statusCheckRollup"`
}

// ghStatus is either a CheckRun (status, conclusion) or a StatusContext
// (state).
type ghStatus struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// LatestPullRequest returns the most recently created pull request whose head
// is branch of repo itself, in any state, or nil when there is none. Pull
// requests from forks that happen to use the same branch name are ignored.
func LatestPullRequest(repo Repo, branch string) (*PullRequest, error) {
	out, err := run("pr", "list",
		"--repo", repo.String(),
		"--head", branch,
		"--state", "all",
		"--limit", "20",
		"--json", "number,state,isDraft,url,headRefOid,isCrossRepository,createdAt,statusCheckRollup")
	if err != nil {
		return nil, err
	}

	var prs []ghPullRequest
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}
	sort.SliceStable(prs, func(i, j int) bool { return prs[i].CreatedAt.After(prs[j].CreatedAt) })

	for _, pr := range prs {
		if pr.IsCrossRepository {
			continue
		}
		return &PullRequest{
			Number: pr.Number,
			State:  strings.ToLower(pr.State),
			Draft:  pr.IsDraft,
			URL:    pr.URL,
			Head:   pr.HeadRefOid,
			Checks: summarizeChecks(pr.StatusCheckRollup),
		}, nil
	}
	return nil, nil
}

// LatestPullRequestFor is LatestPullRequest for the GitHub repository the
// origin of the git repository at repoDir points at.
func LatestPullRequestFor(repoDir, branch string) (*PullRequest, error) {
	repo, err := RepoFromOrigin(repoDir)
	if err != nil {
		return nil, err
	}
	return LatestPullRequest(repo, branch)
}

// summarizeChecks folds status checks into one word: failing when any failed,
// else pending when any has not finished, else passing.
func summarizeChecks(statuses []ghStatus) string {
	if len(statuses) == 0 {
		return ""
	}
	pending := false
	for _, s := range statuses {
		switch {
		case s.State != "": // StatusContext
			switch s.State {
			case "FAILURE", "ERROR":
				return ChecksFailing
			case "PENDING", "EXPECTED":
				pending = true
			}
		case s.Status != "COMPLETED": // CheckRun still queued or running
			pending = true
		default:
			switch s.Conclusion {
			case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
				return ChecksFailing
			}
		}
	}
	if pending {
		return ChecksPending
	}
	return ChecksPassing
}
