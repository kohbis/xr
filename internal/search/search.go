package search

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kohbis/xr/internal/config"
)

type Options struct {
	RepoFilter []string
	Pattern    string
	Glob       string
	Context    int
	IgnoreCase bool
	UseRegex   bool

	// OnRepoError is called when one repository could not be searched. The
	// search continues with the remaining repositories. When nil, such errors
	// are dropped.
	OnRepoError func(repo string, err error)
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
		if len(opts.RepoFilter) > 0 && !slices.Contains(opts.RepoFilter, repo.Name) {
			continue
		}

		repoPath := filepath.Join(wsDir, repo.Path)
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			continue
		}

		repoMatches, err := searchRepo(repo.Name, repoPath, opts)
		if err != nil {
			if opts.OnRepoError != nil {
				opts.OnRepoError(repo.Name, err)
			}
			continue
		}
		matches = append(matches, repoMatches...)
	}

	return matches, nil
}

// searchRepo searches one repository with whichever engine is available. Both
// engines are given the same file list, so the results do not depend on
// ripgrep being installed.
func searchRepo(repoName, repoPath string, opts Options) ([]Match, error) {
	files, err := listFiles(repoPath, opts.Glob)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	var matches []Match
	if isRipgrepAvailable() {
		matches, err = searchWithRipgrep(repoName, repoPath, files, opts)
	} else {
		matches, err = searchBuiltin(repoName, repoPath, files, opts)
	}
	if err != nil {
		return nil, err
	}

	// ripgrep searches the batch in parallel and reports files in whatever
	// order the workers finish, so order the results here. Both engines then
	// produce the same sequence, and repeated runs produce it identically.
	slices.SortStableFunc(matches, func(a, b Match) int {
		if a.File != b.File {
			return cmp.Compare(a.File, b.File)
		}
		return cmp.Compare(a.Line, b.Line)
	})
	return matches, nil
}

// isRipgrepAvailable reports whether the ripgrep binary can be used. It is a
// variable so tests can exercise both engines on the same fixture.
var isRipgrepAvailable = func() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

// Field separators handed to ripgrep in place of ":" and "-". Both are
// characters that cannot occur in a path or in a line of text, so the output
// parses unambiguously no matter what the file names or matches contain.
const (
	ripgrepMatchSep   = "\x1f"
	ripgrepContextSep = "\x1e"
)

// ripgrepArgvBudget caps the bytes of file paths passed to one ripgrep call,
// well below the system limit on argument size, so a repository with many
// files is searched in several batches rather than failing to start.
const ripgrepArgvBudget = 64 << 10

func searchWithRipgrep(repoName, repoPath string, files []string, opts Options) ([]Match, error) {
	base := []string{
		"--line-number",
		"--with-filename",
		"--no-heading",
		"--color=never",
		"--field-match-separator=" + ripgrepMatchSep,
		"--field-context-separator=" + ripgrepContextSep,
	}
	if opts.IgnoreCase {
		base = append(base, "--ignore-case")
	}
	if opts.Context > 0 {
		base = append(base, fmt.Sprintf("--context=%d", opts.Context))
	}
	if !opts.UseRegex {
		base = append(base, "--fixed-strings")
	}
	// -e keeps a pattern that starts with a dash from being read as a flag.
	base = append(base, "-e", opts.Pattern, "--")

	var matches []Match
	for _, batch := range batchFiles(files, ripgrepArgvBudget) {
		cmd := exec.Command("rg", append(slices.Clone(base), batch...)...)
		// Paths are relative, so ripgrep reports them relative too.
		cmd.Dir = repoPath
		out, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				continue // no matches in this batch
			}
			return nil, fmt.Errorf("ripgrep: %w", err)
		}
		matches = append(matches, parseRipgrepOutput(repoName, string(out))...)
	}
	return matches, nil
}

// batchFiles splits paths into groups whose combined length stays within
// budget, so each ripgrep invocation has a bounded argument list.
func batchFiles(files []string, budget int) [][]string {
	var batches [][]string
	var current []string
	size := 0
	for _, f := range files {
		// A single path longer than the budget still has to go somewhere.
		if len(current) > 0 && size+len(f)+1 > budget {
			batches = append(batches, current)
			current, size = nil, 0
		}
		current = append(current, f)
		size += len(f) + 1
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// parseRipgrepOutput turns ripgrep's output into matches. Each line is
// "path SEP line SEP content", with the separator saying whether it is a match
// or a context line. Anything else — the "--" between context groups, or the
// note ripgrep prints for a binary file — does not have that shape and is
// dropped.
func parseRipgrepOutput(repoName, out string) []Match {
	var matches []Match
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		m, ok := parseRipgrepLine(repoName, line)
		if !ok {
			continue
		}
		matches = append(matches, m)
	}
	return matches
}

func parseRipgrepLine(repoName, line string) (Match, bool) {
	sep, isContext := ripgrepMatchSep, false
	if !strings.Contains(line, ripgrepMatchSep) {
		sep, isContext = ripgrepContextSep, true
	}

	parts := strings.SplitN(line, sep, 3)
	if len(parts) < 3 {
		return Match{}, false
	}
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return Match{}, false
	}
	return Match{
		Repo:      repoName,
		File:      parts[0],
		Line:      lineNum,
		Content:   parts[2],
		IsContext: isContext,
	}, true
}

func searchBuiltin(repoName, repoPath string, files []string, opts Options) ([]Match, error) {
	patternStr := opts.Pattern
	if !opts.UseRegex {
		patternStr = regexp.QuoteMeta(patternStr)
	}
	flags := ""
	if opts.IgnoreCase {
		flags = "(?i)"
	}
	pattern, err := regexp.Compile(flags + patternStr)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	var matches []Match
	for _, rel := range files {
		fileMatches, err := searchFile(repoName, rel, filepath.Join(repoPath, filepath.FromSlash(rel)), pattern, opts.Context)
		if err != nil {
			continue
		}
		matches = append(matches, fileMatches...)
	}
	return matches, nil
}

// searchFile returns the matches in one file, along with the requested
// context lines. Binary files are skipped, matching what ripgrep reports for
// them.
func searchFile(repoName, rel, filePath string, pattern *regexp.Regexp, contextLines int) ([]Match, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if isBinary(content) {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	var matches []Match
	emittedLines := make(map[int]bool)
	for i, line := range lines {
		if !pattern.MatchString(line) {
			continue
		}
		start := max(0, i-contextLines)
		end := min(len(lines)-1, i+contextLines)
		for j := start; j <= end; j++ {
			if emittedLines[j] {
				continue
			}
			matches = append(matches, Match{
				Repo:      repoName,
				File:      rel,
				Line:      j + 1,
				Content:   lines[j],
				IsContext: j != i,
			})
			emittedLines[j] = true
		}
	}
	return matches, nil
}
