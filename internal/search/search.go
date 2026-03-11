package search

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kohbis/xr/internal/config"
	"github.com/kohbis/xr/internal/output"
)

type Options struct {
	RepoFilter []string
	Pattern    string
	Glob       string
	Context    int
	IgnoreCase bool
	UseRegex   bool
}

type Match struct {
	Repo      string
	File      string
	Content   string
	Line      int
	IsContext bool
}

func Search(cfg *config.Config, wsDir string, opts Options) ([]Match, error) {
	var matches []Match

	for _, repo := range cfg.Repositories {
		if len(opts.RepoFilter) > 0 && !contains(opts.RepoFilter, repo.Name) {
			continue
		}

		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		repoMatches, err := searchRepo(repo.Name, repoPath, opts)
		if err != nil {
			output.PrintWarning(fmt.Sprintf("searching %s: %v", repo.Name, err))
			continue
		}
		matches = append(matches, repoMatches...)
	}

	return matches, nil
}

func searchRepo(repoName, repoPath string, opts Options) ([]Match, error) {
	if isRipgrepAvailable() {
		return searchWithRipgrep(repoName, repoPath, opts)
	}
	return searchBuiltin(repoName, repoPath, opts)
}

func isRipgrepAvailable() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

func searchWithRipgrep(repoName, repoPath string, opts Options) ([]Match, error) {
	args := []string{"--line-number", "--no-heading", "--color=never"}

	if opts.IgnoreCase {
		args = append(args, "--ignore-case")
	}
	if opts.Context > 0 {
		args = append(args, fmt.Sprintf("--context=%d", opts.Context))
	}
	if opts.Glob != "" {
		args = append(args, "--glob", opts.Glob)
	}
	if !opts.UseRegex {
		args = append(args, "--fixed-strings")
	}

	args = append(args, opts.Pattern, repoPath)

	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil // no matches
		}
		return nil, fmt.Errorf("ripgrep: %w", err)
	}

	return parseRipgrepOutput(repoName, repoPath, string(out), opts.Context > 0)
}

func parseRipgrepOutput(repoName, repoPath, output string, hasContext bool) ([]Match, error) {
	var matches []Match

	for _, line := range strings.Split(output, "\n") {
		if line == "" || line == "--" {
			continue
		}

		isContext := false
		sep := ":"
		if hasContext && strings.Contains(line, "-") {
			parts := strings.SplitN(line, "-", 3)
			if len(parts) == 3 {
				if isFilePath(parts[0]) {
					isContext = true
					sep = "-"
				}
			}
		}

		parts := strings.SplitN(line, sep, 3)
		if len(parts) < 3 {
			continue
		}

		filePath := strings.TrimPrefix(parts[0], repoPath+"/")
		lineNum := 0
		if _, err := fmt.Sscanf(parts[1], "%d", &lineNum); err != nil {
			continue
		}
		content := parts[2]

		matches = append(matches, Match{
			Repo:      repoName,
			File:      filePath,
			Line:      lineNum,
			Content:   content,
			IsContext: isContext,
		})
	}

	return matches, nil
}

func isFilePath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, ".")
}

func searchBuiltin(repoName, repoPath string, opts Options) ([]Match, error) {
	var pattern *regexp.Regexp
	var err error

	patternStr := opts.Pattern
	if !opts.UseRegex {
		patternStr = regexp.QuoteMeta(patternStr)
	}

	flags := ""
	if opts.IgnoreCase {
		flags = "(?i)"
	}

	pattern, err = regexp.Compile(flags + patternStr)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	var matches []Match

	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if opts.Glob != "" {
			matched, globErr := filepath.Match(opts.Glob, info.Name())
			if globErr != nil || !matched {
				return nil
			}
		}

		fileMatches, err := searchFile(repoName, repoPath, path, pattern, opts.Context)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})

	return matches, err
}

func searchFile(repoName, repoPath, filePath string, pattern *regexp.Regexp, contextLines int) ([]Match, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	relPath := strings.TrimPrefix(filePath, repoPath+"/")
	var matches []Match
	emittedLines := make(map[int]bool)

	for i, line := range lines {
		if pattern.MatchString(line) {
			start := max(0, i-contextLines)
			end := min(len(lines)-1, i+contextLines)

			for j := start; j <= end; j++ {
				if !emittedLines[j] {
					matches = append(matches, Match{
						Repo:      repoName,
						File:      relPath,
						Line:      j + 1,
						Content:   lines[j],
						IsContext: j != i,
					})
					emittedLines[j] = true
				}
			}
		}
	}

	return matches, nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
