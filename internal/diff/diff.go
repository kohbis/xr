package diff

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/git"
	"github.com/kohbis/xr/internal/output"
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

type HistoryResult struct {
	Repo  string   `json:"repo"`
	Lines []string `json:"lines"`
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
		err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && info.Name() == fileName {
				found = append(found, path)
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

func SearchPattern(cfg *config.Config, wsDir, pattern string, repoFilter []string) (map[string][]PatternOccurrence, error) {
	result := make(map[string][]PatternOccurrence)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}
		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		var occurrences []PatternOccurrence
		err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()

			relPath := strings.TrimPrefix(path, repoPath+"/")
			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if re.MatchString(line) {
					occurrences = append(occurrences, PatternOccurrence{
						Repo:    repo.Name,
						File:    relPath,
						Line:    lineNum,
						Content: line,
					})
				}
			}
			return nil
		})
		if err != nil {
			output.PrintWarning(fmt.Sprintf("searching %s: %v", repo.Name, err))
			continue
		}

		result[repo.Name] = occurrences
	}

	return result, nil
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

// SearchHistory runs git log --grep in each repository (optionally limited by repoFilter).
func SearchHistory(cfg *config.Config, wsDir, query string, repoFilter []string) error {
	results, err := SearchHistoryResults(cfg, wsDir, query, repoFilter)
	if err != nil {
		return err
	}
	for _, repoRes := range results {
		output.PrintRepoHeader(repoRes.Repo)
		if len(repoRes.Lines) == 0 {
			fmt.Printf("  (no matches)\n")
			continue
		}
		fmt.Print(strings.Join(repoRes.Lines, "\n"))
		fmt.Println()
	}
	return nil
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
				Lines: []string{"(no git history available)"},
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

// GitDiff runs git diff in each repository workspace directory, forwarding args to git diff.
// Use an empty repoFilter to include all configured repos that exist on disk.
func GitDiff(cfg *config.Config, wsDir string, repoFilter []string, gitArgs []string) error {
	gitCmd := append([]string{"-c", "core.pager=cat", "diff"}, gitArgs...)

	for _, repo := range cfg.Repositories {
		if !repoMatchesFilter(repoFilter, repo.Name) {
			continue
		}

		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		output.PrintRepoHeader(repo.Name)

		out, err := git.RunCombinedOutput(repoPath, gitCmd...)
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				fmt.Print(string(out))
				continue
			}
			output.PrintWarning(fmt.Sprintf("git diff in %s: %v\n%s", repo.Name, err, string(out)))
			continue
		}
		fmt.Print(string(out))
	}

	return nil
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
