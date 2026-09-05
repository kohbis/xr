package diff

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
)

type FileComparison struct {
	FileName string
	Repos    []RepoFile
}

type RepoFile struct {
	Repo    string
	Path    string
	Content string
}

type PatternOccurrence struct {
	Repo    string
	File    string
	Content string
	Line    int
}

// PatternResult holds the occurrences of a pattern in one repository. Error
// is set when the repository could not be scanned; Matches is then empty.
type PatternResult struct {
	Repo    string              `json:"repo"`
	Matches []PatternOccurrence `json:"matches"`
	Error   string              `json:"error,omitempty"`
}

type HistoryResult struct {
	Repo  string   `json:"repo"`
	Lines []string `json:"lines"`
	// Error is set when git log could not be run in the repository.
	Error string `json:"error,omitempty"`
}

// GitDiffResult is the output of git diff in one repository.
type GitDiffResult struct {
	Repo   string `json:"repo"`
	Output string `json:"output"`
	// Error is set when git diff failed; Output then holds what git printed.
	Error string `json:"error,omitempty"`
}

func CompareFile(cfg *config.Config, wsDir, fileName string, repoFilter []string) ([]FileComparison, error) {
	var comparisons []FileComparison
	var repoFiles []RepoFile

	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}
		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		var found []string
		err := walkFiles(repoPath, walkOptions{}, func(rel, abs string) error {
			if path.Base(rel) == fileName {
				found = append(found, abs)
			}
			return nil
		})
		if err != nil {
			continue
		}

		for _, f := range found {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			relPath := strings.TrimPrefix(f, repoPath+"/")
			repoFiles = append(repoFiles, RepoFile{
				Repo:    repo.Name,
				Path:    relPath,
				Content: string(content),
			})
		}
	}

	if len(repoFiles) >= 2 {
		comparisons = append(comparisons, FileComparison{
			FileName: fileName,
			Repos:    repoFiles,
		})
	}

	return comparisons, nil
}

// SearchPattern reports where pattern occurs in each repository, in the order
// the repositories are configured. Repositories missing from the workspace are
// left out; a repository that could not be scanned is reported with Error set.
func SearchPattern(cfg *config.Config, wsDir, pattern string, repoFilter []string) ([]PatternResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	var results []PatternResult
	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}
		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		result := PatternResult{Repo: repo.Name, Matches: []PatternOccurrence{}}
		err := walkFiles(repoPath, walkOptions{skipHidden: true}, func(rel, abs string) error {
			f, err := os.Open(abs)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()

			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if re.MatchString(line) {
					result.Matches = append(result.Matches, PatternOccurrence{
						Repo:    repo.Name,
						File:    rel,
						Line:    lineNum,
						Content: line,
					})
				}
			}
			return nil
		})
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}

	return results, nil
}

func repoMatchesFilter(filter []string, name string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == name {
			return true
		}
	}
	return false
}

func SearchHistoryResults(cfg *config.Config, wsDir, query string, repoFilter []string) ([]HistoryResult, error) {
	var results []HistoryResult
	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}

		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		out, err := git.RunOutput(repoPath, "log", "--all", "--oneline", "--grep="+query)
		if err != nil {
			results = append(results, HistoryResult{
				Repo:  repo.Name,
				Lines: []string{},
				Error: fmt.Sprintf("git log: %v", err),
			})
			continue
		}
		lines := []string{}
		if len(out) > 0 {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if strings.TrimSpace(line) != "" {
					lines = append(lines, line)
				}
			}
		}
		results = append(results, HistoryResult{Repo: repo.Name, Lines: lines})
	}
	return results, nil
}

// GitDiff runs git diff in each repository workspace directory, forwarding
// args to git diff, and returns the output per repository in configuration
// order. Use an empty repoFilter to include all configured repos that exist on
// disk. git's exit status 1 (differences found) is not an error.
func GitDiff(cfg *config.Config, wsDir string, repoFilter []string, gitArgs []string) []GitDiffResult {
	gitCmd := append([]string{"-c", "core.pager=cat", "diff"}, gitArgs...)

	var results []GitDiffResult
	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}

		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		result := GitDiffResult{Repo: repo.Name}
		out, err := git.RunCombinedOutput(repoPath, gitCmd...)
		result.Output = string(out)
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				result.Error = fmt.Sprintf("git diff: %v", err)
			}
		}
		results = append(results, result)
	}
	return results
}

func DiffFiles(file1, file2 RepoFile) (string, error) {
	tmpDir := os.TempDir()

	f1Path := filepath.Join(tmpDir, "xr_diff_a_"+filepath.Base(file1.Path))
	f2Path := filepath.Join(tmpDir, "xr_diff_b_"+filepath.Base(file2.Path))

	if err := os.WriteFile(f1Path, []byte(file1.Content), 0600); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(f1Path) }()

	if err := os.WriteFile(f2Path, []byte(file2.Content), 0600); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(f2Path) }()

	cmd := exec.Command("diff", "-u",
		fmt.Sprintf("--label=%s:%s", file1.Repo, file1.Path),
		fmt.Sprintf("--label=%s:%s", file2.Repo, file2.Path),
		f1Path, f2Path)
	out, _ := cmd.Output() // diff returns exit code 1 when files differ

	return string(out), nil
}
