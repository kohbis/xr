package structure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var depFiles = map[string]string{
	"go.mod":           "Go",
	"go.sum":           "Go",
	"package.json":     "Node.js",
	"Cargo.toml":       "Rust",
	"requirements.txt": "Python",
	"pyproject.toml":   "Python",
	"pom.xml":          "Java",
	"build.gradle":     "Java",
	"Gemfile":          "Ruby",
	"composer.json":    "PHP",
}

type RepoInfo struct {
	LastUpdated time.Time
	Children    []*Node
	Name        string
	Path        string
	Language    string
	Branch      string
	Commit      string
	Dirty       bool
	FileCount   int
}

type Node struct {
	Children []*Node
	Name     string
	IsDir    bool
	IsDep    bool
}

type gitIgnoreChecker struct {
	root  string
	cache map[string]bool
}

func newGitIgnoreChecker(root string) *gitIgnoreChecker {
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err != nil {
		return nil
	}
	return &gitIgnoreChecker{
		root:  root,
		cache: make(map[string]bool),
	}
}

func (c *gitIgnoreChecker) isIgnored(relPath string, isDir bool) bool {
	if c == nil {
		return false
	}

	key := relPath
	if isDir {
		key += "/"
	}

	if v, ok := c.cache[key]; ok {
		return v
	}

	args := []string{"check-ignore", "-q", "--", key}
	cmd := exec.Command("git", args...)
	cmd.Dir = c.root

	// Exit codes: 0 = ignored, 1 = not ignored, 128 = error.
	err := cmd.Run()
	ignored := err == nil
	c.cache[key] = ignored
	return ignored
}

func AnalyzeRepo(name, repoPath string, maxDepth int) (*RepoInfo, error) {
	info := &RepoInfo{
		Name: name,
		Path: repoPath,
	}

	root := &Node{Name: name, IsDir: true}
	var lastMod time.Time
	fileCount := 0
	language := ""

	branch, commit, dirty := gitSummary(repoPath)

	ignore := newGitIgnoreChecker(repoPath)
	err := walkDir(repoPath, root, repoPath, ignore, 0, maxDepth, &fileCount, &lastMod, &language)
	if err != nil {
		return nil, err
	}

	info.FileCount = fileCount
	info.LastUpdated = lastMod
	info.Language = language
	info.Branch = branch
	info.Commit = commit
	info.Dirty = dirty
	info.Children = root.Children

	return info, nil
}

func gitSummary(repoPath string) (branch string, commit string, dirty bool) {
	// If this isn't a git repo, just return empty metadata.
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return "", "", false
	}

	branch = strings.TrimSpace(string(gitOut(repoPath, "rev-parse", "--abbrev-ref", "HEAD")))
	commit = strings.TrimSpace(string(gitOut(repoPath, "rev-parse", "--short", "HEAD")))
	// `git status --porcelain` is empty when clean.
	dirty = strings.TrimSpace(string(gitOut(repoPath, "status", "--porcelain"))) != ""
	return branch, commit, dirty
}

func gitOut(repoPath string, args ...string) []byte {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

func walkDir(dirPath string, node *Node, rootPath string, ignore *gitIgnoreChecker, depth, maxDepth int, fileCount *int, lastMod *time.Time, language *string) error {
	if maxDepth > 0 && depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Sort: dirs first, then files
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()

		if strings.HasPrefix(name, ".") {
			continue
		}

		rel, err := filepath.Rel(rootPath, filepath.Join(dirPath, name))
		if err == nil && ignore.isIgnored(rel, entry.IsDir()) {
			continue
		}

		childPath := filepath.Join(dirPath, name)
		isDep := false

		if lang, ok := depFiles[name]; ok {
			isDep = true
			if *language == "" {
				*language = lang
			}
		}

		child := &Node{
			Name:  name,
			IsDir: entry.IsDir(),
			IsDep: isDep,
		}

		if entry.IsDir() {
			if err := walkDir(childPath, child, rootPath, ignore, depth+1, maxDepth, fileCount, lastMod, language); err != nil {
				continue
			}
		} else {
			*fileCount++
			info, err := entry.Info()
			if err == nil && info.ModTime().After(*lastMod) {
				*lastMod = info.ModTime()
			}
		}

		node.Children = append(node.Children, child)
	}

	return nil
}

func PrintTree(info *RepoInfo) {
	fmt.Printf("%s", info.Name)
	if info.Language != "" {
		fmt.Printf(" [%s]", info.Language)
	}

	meta := []string{}
	if info.Branch != "" {
		meta = append(meta, info.Branch)
	}
	if info.Commit != "" {
		meta = append(meta, info.Commit)
	}
	if info.Dirty {
		meta = append(meta, "dirty")
	}
	if len(meta) > 0 {
		fmt.Printf(" (%s)", strings.Join(meta, " "))
	}
	fmt.Println()

	printNodes(info.Children, "")
}

func printNodes(nodes []*Node, prefix string) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}
		fmt.Printf("%s%s%s\n", prefix, connector, node.Name)

		if node.IsDir && len(node.Children) > 0 {
			printNodes(node.Children, childPrefix)
		}
	}
}
