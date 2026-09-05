package diff

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kohbis/xr/internal/git"
)

// walkOptions controls which files walkFiles visits.
type walkOptions struct {
	// skipHidden leaves out files and directories whose name starts with a dot.
	skipHidden bool
}

// walkFiles calls fn for each regular file of the repository at repoPath,
// with the path relative to repoPath, in sorted order.
//
// In a git repository the file set comes from git itself (tracked files plus
// untracked files that are not ignored), so build output, dependency trees and
// the .git directory never take part in a comparison. Outside git it falls back
// to walking the directory, still skipping .git.
func walkFiles(repoPath string, opts walkOptions, fn func(rel, abs string) error) error {
	if files, err := git.ListFiles(repoPath); err == nil {
		for _, rel := range files {
			if opts.skipHidden && isHiddenPath(rel) {
				continue
			}
			abs := filepath.Join(repoPath, filepath.FromSlash(rel))
			info, err := os.Stat(abs)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if err := fn(rel, abs); err != nil {
				return err
			}
		}
		return nil
	}

	return filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == repoPath {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if name == ".git" || (opts.skipHidden && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if opts.skipHidden && strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		return fn(filepath.ToSlash(rel), path)
	})
}

// isHiddenPath reports whether any component of the slash-separated path
// starts with a dot.
func isHiddenPath(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
