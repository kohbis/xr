package structure

import (
	"fmt"
	"os"
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
	Name        string
	Path        string
	Language    string
	FileCount   int
	LastUpdated time.Time
	Children    []*Node
}

type Node struct {
	Name     string
	IsDir    bool
	IsDep    bool
	Children []*Node
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

	err := walkDir(repoPath, root, repoPath, 0, maxDepth, &fileCount, &lastMod, &language)
	if err != nil {
		return nil, err
	}

	info.FileCount = fileCount
	info.LastUpdated = lastMod
	info.Language = language
	info.Children = root.Children

	return info, nil
}

func walkDir(dirPath string, node *Node, rootPath string, depth, maxDepth int, fileCount *int, lastMod *time.Time, language *string) error {
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
			if err := walkDir(childPath, child, rootPath, depth+1, maxDepth, fileCount, lastMod, language); err != nil {
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

func PrintTree(info *RepoInfo, showDepsOnly bool) {
	fmt.Printf("%s", info.Name)
	if info.Language != "" {
		fmt.Printf(" [%s]", info.Language)
	}
	fmt.Printf(" (%d files)\n", info.FileCount)

	printNodes(info.Children, "", showDepsOnly)
}

func printNodes(nodes []*Node, prefix string, showDepsOnly bool) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		if showDepsOnly && !node.IsDir && !node.IsDep {
			continue
		}

		depMark := ""
		if node.IsDep {
			depMark = " *"
		}

		fmt.Printf("%s%s%s%s\n", prefix, connector, node.Name, depMark)

		if node.IsDir && len(node.Children) > 0 {
			printNodes(node.Children, childPrefix, showDepsOnly)
		}
	}
}
